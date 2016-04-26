//this is a generic class which is used to pass url and get data from the end point
//on error , this is not yet handled
//Ceated by:Jiten
//Created On:April-8th-2016

function getDataFromRESTFulEndPoint(url)
		 {
					var returnData;
                    $.ajax(
                    {
                        dataType: 'json',
						 crossDomain:true,
						 headers: {
                            Accept:'application/json',
							'Access-Control-Allow-Origin': '*',
							'Access-Control-Allow-Methods': 'GET, OPTIONS'
							},
                        type:'GET',
                        url:url,
						async: false,
                        success: function(data)
                        {	
						    returnData=data;
						},
                        error: function(data)
                        {
							alert("error");
							if(typeof data !== "undefined"){
								// Handle exception here...
							}
							console.log(data);
						  //Error data to be handled.
						   returnData = data;
                           return returnData;
                        }
                    });
					
					return returnData;
    } 