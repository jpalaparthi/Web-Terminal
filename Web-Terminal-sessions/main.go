// AwordAday project main.go
package main

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kr/pty"
)

const STATIC_URL string = "/static/"

const STATIC_ROOT string = "static/"

type Context struct {
	Title   string
	Static  string
	Command string
}

var (
	c   *exec.Cmd
	f   *os.File
	fo  *os.File
	err error
	tm  *TerminalManager
)

type Terminal struct {
	Cmd        *exec.Cmd
	Tfile      *os.File //file name
	Fout       string   //file name
	SessionID  string
	CopyStatus bool
}

type TerminalManager struct {
	Terminals []Terminal
}

var readdata chan []byte

func main() {

	CopyDir("t-files/", "t-history")
	tm = &TerminalManager{}
	tm.Terminals = make([]Terminal, 0)
	//go tm.Copy()
	log.Println("service started")
	http.HandleFunc(STATIC_URL, StaticHandler)
	http.HandleFunc("/", home)
	http.HandleFunc("/home/", home)
	http.HandleFunc("/terminal/", terminal)

	http.ListenAndServe(":8089", nil)
	fmt.Println("is kill working?")
}

func StaticHandler(w http.ResponseWriter, req *http.Request) {
	static_file := req.URL.Path[len(STATIC_URL):]
	if len(static_file) != 0 {
		f, err := http.Dir(STATIC_ROOT).Open(static_file)
		if err == nil {
			content := io.ReadSeeker(f)
			http.ServeContent(w, req, static_file, time.Now(), content)
			return
		}
	}
	http.NotFound(w, req)
}

func home(w http.ResponseWriter, r *http.Request) {
	render(w, "home", Context{Title: "Home"})
}

func render(w http.ResponseWriter, tmpl string, context Context) {
	context.Static = STATIC_URL
	tmpl_list := []string{"templates/base.html",
		fmt.Sprintf("templates/%s.html", tmpl)}
	t, err := template.ParseFiles(tmpl_list...)
	if err != nil {
		log.Print("template parsing error: ", err)
	}
	err = t.Execute(w, context)
	if err != nil {
		log.Print("template executing error: ", err)
	}
}

func terminal(w http.ResponseWriter, r *http.Request) {
	var t Terminal
	//var tr *Terminal
	var session string
	cookie, err := r.Cookie("terminalsession")
	if err != nil {
		Logerr(err)
	} else {
		session = cookie.Value
	}

	if session == "" {
		session = GetSessionID()
		//expiration := time.Now().Add(365 * 24 * time.Hour)
		//expiration := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{Name: "terminalsession", Value: session, MaxAge: -1}
		http.SetCookie(w, &cookie)

		t, err = New(session, "bash")

		tm.AddTerminal(t)

	} else {
		t, err = tm.GetTerminalBySession(session)
		if err != nil {
			t, err = New(session, "bash")

			err = tm.AddTerminal(t)

		}

	}

	if r.Method == "GET" {
		fmt.Println("session in get", session)
		render(w, "terminal", Context{Title: "Terminal", Command: ReadFile("t-files/" + session + ".cd")})
	}

	if r.Method == "POST" {
		err = r.ParseForm()
		t, err = tm.GetTerminalBySession(session)
		fmt.Println(t)
		fmt.Println("session in post", session)
		Logerr(err)
		v := strings.Trim(r.PostFormValue("txtcommand"), " ")
		ls := strings.LastIndex(v, "$")
		if v != "" {
			v = v[ls+1:]
		}
		_, err = t.Write([]byte(v))
		_, err = t.Write([]byte{13})
		go t.Copy()
		Logerr(err)
		r.Method = "GET"
		time.Sleep(time.Second * 1)
		http.Redirect(w, r, "/terminal/", 301)
	}
}

func ReadFile(fn string) string {
	fl, _ := os.Open(fn)
	fl.Close()
	buf, _ := ioutil.ReadFile(fn)
	return string(buf)
}

//Log error is to log errors in the system.
//all errors in the server are logged to logger.
//logger can be changed to file or any io.writer supported interface
func Logerr(err error) {
	if err != nil {
		log.Println(err)
	}
}

func Copy(src io.Reader, dest_path string) {
	r := bufio.NewReader(src)
	for {
		if Exists(dest_path) {
			b, err := r.ReadByte()
			if err == nil {
				WriteTo(string(b), dest_path)
			}
		}
	}
}

func WriteTo(s string, path string) (err error) {
	fo, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	Logerr(err)
	fo.WriteString(s)
	fo.Close()
	return nil
}

func Create(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return err
	} else {
		f.Close()
		return nil
	}
}

// Exists reports whether the named file or directory exists.
func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (tm *TerminalManager) GetTerminalBySession(sessionId string) (t Terminal, e error) {
	if tm.Terminals == nil {
		return t, errors.New("no added terminals")
	}
	for i := 0; i < len(tm.Terminals); i++ {
		tv := tm.Terminals[i]
		if tv.SessionID == sessionId {
			return tm.Terminals[i], nil
		}
	}

	return t, errors.New("no added terminals")
}

func (tm *TerminalManager) AddTerminal(t Terminal) (e error) {
	if tm == nil {
		return errors.New("terminal manager is nil.")
	}
	tm.Terminals = append(tm.Terminals, t)
	return e
}

//Kills all terminals in the in the manager
//ideally to be called during main exit
func (tm *TerminalManager) Kill() {
	go func() {
		for _, t := range tm.Terminals {
			t.Kill()
		}
	}()
}

//Concurrently execute all terminal copy process inside the manager
func (tm *TerminalManager) Copy() {
	for {
		for _, t := range tm.Terminals {
			if !t.CopyStatus && t.SessionID != "" {
				t.Copy()
				fmt.Println("after copy?")
			}
		}
	}
}

//Creates a new terminal object..
//Takes sessionID as input parameter and make output and src files accordingly
func New(sessionId, cmd string) (Terminal, error) {
	t := Terminal{}
	t.SessionID = sessionId
	t.Fout = "t-files/" + sessionId + ".cd"
	t.Cmd = exec.Command(cmd)
	t.Tfile, err = pty.Start(t.Cmd) //start the stream but will never be stopped..as such
	err = Create(t.Fout)
	return t, err
}

//Coping terminal data.from stream to the file specified
//This is supposed to be the concurrent stuff
//Reason is stream is not closed.. data would be coming and write to the file.
func (t *Terminal) Copy() {
	if t.CopyStatus == false {
		for i := 0; i < len(tm.Terminals); i++ {
			tv := tm.Terminals[i]
			if tv.SessionID == t.SessionID {
				tm.Terminals[i].CopyStatus = true
				break
				//return tm.Terminals[i], nil
			}
		}
		//		t.CopyStatus = true
		r := bufio.NewReader(t.Tfile)
		for {
			if Exists(t.Fout) {
				b, err := r.ReadByte()
				if err == nil {
					err = WriteTo(string(b), t.Fout)
				}
			}
		}
	}
}

//Write data to the terminal stream.
func (t *Terminal) Write(c []byte) (int, error) {
	return t.Tfile.Write(c)
}

//Kills receiver terminal.
func (t *Terminal) Kill() (e error) {
	t.Tfile.Write([]byte{4}) // Writing EOT ascii code.. which is end of the transaction , which ultimately closes the stream..
	t.Tfile.Close()          //Just for the sake of my satisfaction
	return e
}

//To get a GUID based session ID
func GetSessionID() string {
	// generate 32 bits timestamp
	unix32bits := uint32(time.Now().UTC().Unix())

	buff := make([]byte, 12)

	rand.Read(buff)
	return fmt.Sprintf("%x-%x-%x-%x-%x-%x", unix32bits, buff[0:2], buff[2:4], buff[4:6], buff[6:8], buff[8:])
}

func CopyDir(source string, dest string) (err error) {
	// get properties of source dir
	sourceinfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	// create dest dir

	err = os.MkdirAll(dest, sourceinfo.Mode())
	if err != nil {
		return err
	}

	directory, _ := os.Open(source)

	objects, err := directory.Readdir(-1)

	for _, obj := range objects {

		sourcefilepointer := source + "/" + obj.Name()

		destinationfilepointer := dest + "/" + obj.Name()

		if obj.IsDir() {
			// create sub-directories - recursively
			err = CopyDir(sourcefilepointer, destinationfilepointer)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			// perform copy
			err = CopyFile(sourcefilepointer, destinationfilepointer)
			if err != nil {
				fmt.Println(err)
			}
		}

	}
	return
}

//Copies file from source to the destination
//Once copies the file would be deleted from the source
func CopyFile(source string, dest string) (err error) {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}

	defer sourcefile.Close()

	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer destfile.Close()

	_, err = io.Copy(destfile, sourcefile)
	if err == nil {
		sourceinfo, err := os.Stat(source)
		if err != nil {
			err = os.Chmod(dest, sourceinfo.Mode())

		}

	}
	os.RemoveAll(source)

	return
}
