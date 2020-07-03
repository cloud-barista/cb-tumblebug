// Cloud Info Manager's Rest Runtime of CB-Tumblebug.
// The CB-Tumblebug is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Tumblebug Team, 2020.06.

package webadmin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	//cr "github.com/cloud-barista/cb-spider/api-runtime/common-runtime"
)

//var cblog *logrus.Logger

func init() {
	//cblog = config.Cblogger
}

type NameWidth struct {
	Name  string
	Width string
}

func cloudosList() []string {
	resBody, err := getResourceList_JsonByte("cloudos")
	if err != nil {
		common.CBLog.Error(err)
	}
	var info struct {
		ResultList []string `json:"cloudos"`
	}
	json.Unmarshal(resBody, &info)

	return info.ResultList
}

//================ Mainpage
func Mainpage(c echo.Context) error {
	common.CBLog.Info("call frame()")

	htmlStr := `
<html>
  <head>
    <title>CB-Tumblebug Admin Web Tool ....__^..^__....</title>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
  </head>
    <frameset rows="80,*" frameborder="Yes" border=1">
        <frame src="webadmin/top" name="top_frame" scrolling="auto" noresize marginwidth="0" marginheight="0"/>
        <frameset frameborder="Yes" border=1">
            <frame src="webadmin/driver" name="main_frame" scrolling="auto" noresize marginwidth="5" marginheight="0"/> 
        </frameset>
    </frameset>
    <noframes>
    <body>
    
    
    </body>
    </noframes>
</html>
        `

	return c.HTML(http.StatusOK, htmlStr)
}

//================ Menu
func Menu(c echo.Context) error {
	common.CBLog.Info("call top()")

	htmlStr := ` 
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
</head>
<body>
    <br>
    <!-- <table border="0" bordercolordark="#FFFFFF" cellpadding="0" cellspacing="2" bgcolor="#FFFFFF" width="320" style="font-size:small;"> -->
    <table border="0" bordercolordark="#FFFFFF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">      
        <tr bgcolor="#FFFFFF" align="center">
            <td rowspan="2" width="80" bgcolor="#FFFFFF">
                <!-- CB-Tumblebug Logo -->
                <a href="../webadmin" target="_top">
                  <img height="45" width="42" src="https://cloud-barista.github.io/assets/img/frameworks/cb-tumblebug.png" border='0' hspace='0' vspace='1' align="middle">
                </a>
		<font size=1>$$TIME$$</font>	
            </td>

            <td width="100">       
                <a href="ns" target="main_frame">            
                    <font size=2>Namespace</font>
                </a>
            </td>
            <td width="100">       

            </td>
            <td width="100">       
                <a href="image" target="main_frame">            
                    <font size=2>Image</font>
                </a>
            </td>
            <td width="100">       
                <a href="spec" target="main_frame">            
                    <font size=2>Spec</font>
                </a>
            </td>
            <td width="100">       

            </td>
            <td width="100">       
                <a href="tumblebugInfo" target="main_frame">            
                    <font size=2>this spider</font>
                </a>
            </td>
            <td width="100">       
                <a href="https://github.com/cloud-barista/cb-tumblebug" target="_blank">            
                    <font size=2>GitHub</font>
                </a>
            </td> 
	</tr>

        <tr bgcolor="#FFFFFF" align="center">
            <td width="100">
                <a href="vNet" target="main_frame">
                    <font size=2>vNet + Subnet</font>
                </a>
            </td>
            <td width="100">
                <a href="securityGroup" target="_blank">
                    <font size=2>Security Group</font>
                </a>
            </td>
            <td width="100">
                <a href="sshKey" target="_blank">
                    <font size=2>SSH Key</font>
                </a>
            </td>
            <td width="100">       

            </td>
            <td width="100">
                <a href="mcis" target="_blank">
                    <font size=2>MCIS</font>
                </a>
            </td>            
        </tr>

    </table>
</body>
</html>
	`

	htmlStr = strings.ReplaceAll(htmlStr, "$$TIME$$", cr.ShortStartTime)
	return c.HTML(http.StatusOK, htmlStr)
}

func makeSelect_html(onchangeFunctionName string) string {
	strList := cloudosList()

	strSelect := `<select name="text_box" id="1" onchange="` + onchangeFunctionName + `(this)">`
	for _, one := range strList {
		if one == "AWS" {
			strSelect += `<option value="` + one + `" selected>` + one + `</option>`
		} else {
			strSelect += `<option value="` + one + `">` + one + `</option>`
		}
	}

	strSelect += `
		</select>
	`

	return strSelect
}

func getResourceList_JsonByte(resourceName string) ([]byte, error) {
	// cr.ServicePort = ":1323"
	url := "http://localhost" + cr.ServicePort + "/spider/" + resourceName

	// get object list
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	resBody, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	return resBody, err
}

// F5, X ("5", "driver", "deleteDriver()", "2")
func makeActionTR_html(colspan string, f5_href string, delete_href string, fontSize string) string {
	if fontSize == "" {
		fontSize = "2"
	}

	strTR := fmt.Sprintf(`
		<tr bgcolor="#FFFFFF" align="right">
		    <td colspan="%s">
			<a href="%s">
			    <font size=%s><b>&nbsp;F5</b></font>
			</a>
			&nbsp;
			<a href="javascript:%s;">
			    <font size=%s><b>&nbsp;X</b></font>
			</a>
			&nbsp;
		    </td>
		</tr>
       		`, colspan, f5_href, fontSize, delete_href, fontSize)

	return strTR
}

//         fieldName-width
// number, fieldName0-200, fieldName1-400, ... , checkbox
func makeTitleTRList_html(bgcolor string, fontSize string, nameWidthList []NameWidth) string {
	if bgcolor == "" {
		bgcolor = "#DDDDDD"
	}
	if fontSize == "" {
		fontSize = "2"
	}

	// (1) header number field
	strTR := fmt.Sprintf(`
		<tr bgcolor="%s" align="center">
		    <td width="15">
			    <font size=%s><b>&nbsp;#</b></font>
		    </td>
		`, bgcolor, fontSize)

	// (2) header title field
	for _, one := range nameWidthList {
		str := fmt.Sprintf(`
			    <td width="%s">
				    <font size=2>%s</font>
			    </td>
			`, one.Width, one.Name)
		strTR += str
	}

	// (3) header checkbox field
	strTR += `
		    <td width="15">
			    <input type="checkbox" onclick="toggle(this);" />
		    </td>
		</tr>
		`
	return strTR
}

// number, Provider Name, Driver File, Driver Name, checkbox
func makeDriverTRList_html(bgcolor string, height string, fontSize string, infoList []*dim.CloudDriverInfo) string {
	if bgcolor == "" {
		bgcolor = "#FFFFFF"
	}
	if height == "" {
		height = "30"
	}
	if fontSize == "" {
		fontSize = "2"
	}

	// make base TR frame for info list
	strTR := fmt.Sprintf(`
                <tr bgcolor="%s" align="center" height="%s">
                    <td>
                            <font size=%s>$$NUM$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S1$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S2$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S3$$</font>
                    </td>
                    <td>
                        <input type="checkbox" name="check_box" value=$$S3$$>
                    </td>
                </tr>
       		`, bgcolor, height, fontSize, fontSize, fontSize, fontSize)

	strData := ""
	// set data and make TR list
	for i, one := range infoList {
		str := strings.ReplaceAll(strTR, "$$NUM$$", strconv.Itoa(i+1))
		str = strings.ReplaceAll(str, "$$S1$$", one.ProviderName)
		str = strings.ReplaceAll(str, "$$S2$$", one.DriverLibFileName)
		str = strings.ReplaceAll(str, "$$S3$$", one.DriverName)
		strData += str
	}

	return strData
}

// make the string of javascript function
func makeOnchangeDriverProviderFunc_js() string {
	strFunc := `
              function onchangeProvider(source) {
                var providerName = source.value
                document.getElementById('2').value= providerName.toLowerCase() + "-driver-v1.0.so";
                document.getElementById('3').value= providerName.toLowerCase() + "-driver-01";
              }
        `

	return strFunc
}

// make the string of javascript function
func makeCheckBoxToggleFunc_js() string {

	strFunc := `
              function toggle(source) {
                var checkboxes = document.getElementsByName('check_box');
                for (var i = 0; i < checkboxes.length; i++) {
                  checkboxes[i].checked = source.checked;
                }
              }
        `

	return strFunc
}

// make the string of javascript function
func makePostDriverFunc_js() string {

	// curl -X POST http://$RESTSERVER:1323/spider/driver -H 'Content-Type: application/json'  -d '{"DriverName":"aws-driver01","ProviderName":"AWS", "DriverLibFileName":"aws-driver-v1.0.so"}'

	strFunc := `
                function postDriver() {
                        var textboxes = document.getElementsByName('text_box');
			sendJson = '{ "ProviderName" : "$$PROVIDER$$", "DriverLibFileName" : "$$$DRVFILE$$", "DriverName" : "$$NAME$$" }'
                        for (var i = 0; i < textboxes.length; i++) { // @todo make parallel executions
                                switch (textboxes[i].id) {
                                        case "1":
                                                sendJson = sendJson.replace("$$PROVIDER$$", textboxes[i].value);
                                                break;
                                        case "2":
                                                sendJson = sendJson.replace("$$$DRVFILE$$", textboxes[i].value);
                                                break;
                                        case "3":
                                                sendJson = sendJson.replace("$$NAME$$", textboxes[i].value);
                                                break;
                                        default:
                                                break;
                                }
                        }
                        var xhr = new XMLHttpRequest();
                        xhr.open("POST", "$$TUMBLEBUG_SERVER$$/spider/driver", true);
                        xhr.setRequestHeader('Content-Type', 'application/json');
                        //xhr.send(JSON.stringify({ "DriverName": driverName, "ProviderName": providerName, "DriverLibFileName": driverLibFileName}));
			//xhr.send(JSON.stringify(sendJson));
			xhr.send(sendJson);

                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

// make the string of javascript function
func makeDeleteDriverFunc_js() string {
	// curl -X DELETE http://$RESTSERVER:1323/spider/driver/gcp-driver01 -H 'Content-Type: application/json'

	strFunc := `
                function deleteDriver() {
                        var checkboxes = document.getElementsByName('check_box');
                        for (var i = 0; i < checkboxes.length; i++) { // @todo make parallel executions
                                if (checkboxes[i].checked) {
                                        var xhr = new XMLHttpRequest();
                                        xhr.open("DELETE", "$$TUMBLEBUG_SERVER$$/spider/driver/" + checkboxes[i].value, true);
                                        xhr.setRequestHeader('Content-Type', 'application/json');
                                        xhr.send(null);
                                }
                        }
                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

//================ Driver Info Management
// create driver page
func Driver(c echo.Context) error {
	common.CBLog.Info("call driver()")

	// make page header
	htmlStr := ` 
		<html>
		<head>
		    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
		    <script type="text/javascript">
		`
	// (1) make Javascript Function
	htmlStr += makeOnchangeDriverProviderFunc_js()
	htmlStr += makeCheckBoxToggleFunc_js()
	htmlStr += makePostDriverFunc_js()
	htmlStr += makeDeleteDriverFunc_js()

	htmlStr += `
		    </script>
		</head>

		<body>
		    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">      
		`

	// (2) make Table Action TR
	// colspan, f5_href, delete_href, fontSize
	htmlStr += makeActionTR_html("5", "driver", "deleteDriver()", "2")

	// (3) make Table Header TR

	nameWidthList := []NameWidth{
		{"Provider Name", "200"},
		{"Driver Library Name", "300"},
		{"Driver Name", "200"},
	}
	htmlStr += makeTitleTRList_html("#DDDDDD", "2", nameWidthList)

	// (4) make Table info list TR
	// (4-1) get driver info list @todo if empty list
	resBody, err := getResourceList_JsonByte("driver")
	if err != nil {
		common.CBLog.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var info struct {
		//ResultList []*dim.CloudDriverInfo `json:"driver"`
	}
	json.Unmarshal(resBody, &info)

	// (4-2) make Table info list TR
	htmlStr += makeDriverTRList_html("", "", "", info.ResultList)

	// (5) make input field and add
	// attach text box for add
	htmlStr += `
			<tr bgcolor="#FFFFFF" align="center" height="30">
			    <td>
				    <font size=2>#</font>
			    </td>
			    <td>
				<!-- <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="1" value="AWS"> -->
		`
	// Select format of CloudOS  name=text_box, id=1
	htmlStr += makeSelect_html("onchangeProvider")

	htmlStr += `
			    </td>
			    <td>
				<input style="font-size:12px;text-align:center;" type="text" name="text_box" id="2" value="aws-driver-v1.0.so">
			    </td>
			    <td>
				<input style="font-size:12px;text-align:center;" type="text" name="text_box" id="3" value="aws-driver01">
			    </td>
			    <td>
				<a href="javascript:postDriver()">
				    <font size=3><b>+</b></font>
				</a>
			    </td>
			</tr>
		`
	// make page tail
	htmlStr += `
                    </table>
                </body>
                </html>
        `

	//fmt.Println(htmlStr)
	return c.HTML(http.StatusOK, htmlStr)
}

// make the string of javascript function
func makeOnchangeCredentialProviderFunc_js() string {
	strFunc := `
              function onchangeProvider(source) {
                var providerName = source.value
		// for credential info
		switch(providerName) {
		  case "AWS":
			credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
		    break;
		  case "AZURE":
			credentialInfo = '[{"Key":"ClientId", "Value":"XXXX-XXXX"}, {"Key":"ClientSecret", "Value":"xxxx-xxxx"}, {"Key":"TenantId", "Value":"xxxx-xxxx"}, {"Key":"SubscriptionId", "Value":"xxxx-xxxx"}]'
		    break;
		  case "GCP":
			credentialInfo = '[{"Key":"PrivateKey", "Value":"-----BEGIN PRIVATE KEY-----\nXXXX\n-----END PRIVATE KEY-----\n"},{"Key":"ProjectID", "Value":"powerkimhub"}, {"Key":"ClientEmail", "Value":"xxxx@xxxx.iam.gserviceaccount.com"}]'
		    break;
		  case "ALIBABA":
			credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
		    break;
		  case "CLOUDIT":
			credentialInfo = '[{"Key":"IdentityEndpoint", "Value":"http://xxx.xxx.co.kr:9090"}, {"Key":"AuthToken", "Value":"xxxx"}, {"Key":"Username", "Value":"xxxx"}, {"Key":"Password", "Value":"xxxx"}, {"Key":"TenantId", "Value":"tnt0009"}]'
		    break;
		  case "OPENSTACK":
			credentialInfo = '[{"Key":"IdentityEndpoint", "Value":"http://182.252.xxx.xxx:5000/v3"}, {"Key":"Username", "Value":"etri"}, {"Key":"Password", "Value":"xxxx"}, {"Key":"DomainName", "Value":"default"}, {"Key":"ProjectID", "Value":"xxxx"}]'
		    break;
		  case "DOCKER":
			credentialInfo = '[{"Key":"Host", "Value":"http://18.191.xxx.xxx:1004"}, {"Key":"APIVersion", "Value":"v1.38"}]'
		    break;
		  case "CLOUDTWIN":
			credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
		    break;
		  default:
			credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
		}
                document.getElementById('2').value= credentialInfo

		// for credential name
                document.getElementById('3').value= providerName.toLowerCase() + "-credential-01";
              }
        `
	return strFunc
}

// number, Provider Name, Credential Info, Credential Name, checkbox
func makeCredentialTRList_html(bgcolor string, height string, fontSize string, infoList []*cim.CredentialInfo) string {
	if bgcolor == "" {
		bgcolor = "#FFFFFF"
	}
	if height == "" {
		height = "30"
	}
	if fontSize == "" {
		fontSize = "2"
	}

	// make base TR frame for info list
	strTR := fmt.Sprintf(`
                <tr bgcolor="%s" align="center" height="%s">
                    <td>
                            <font size=%s>$$NUM$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S1$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S2$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S3$$</font>
                    </td>
                    <td>
                        <input type="checkbox" name="check_box" value=$$S3$$>
                    </td>
                </tr>
                `, bgcolor, height, fontSize, fontSize, fontSize, fontSize)

	strData := ""
	// set data and make TR list
	for i, one := range infoList {
		str := strings.ReplaceAll(strTR, "$$NUM$$", strconv.Itoa(i+1))
		str = strings.ReplaceAll(str, "$$S1$$", one.ProviderName)
		strKeyList := ""
		for _, kv := range one.KeyValueInfoList {
			strKeyList += kv.Key + ":xxxx, "
		}
		str = strings.ReplaceAll(str, "$$S2$$", strKeyList)
		str = strings.ReplaceAll(str, "$$S3$$", one.CredentialName)
		strData += str
	}

	return strData
}

// make the string of javascript function
func makePostCredentialFunc_js() string {

	// curl -X POST http://$RESTSERVER:1323/spider/credential -H 'Content-Type: application/json' '{"CredentialName":"aws-credential01","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]}'

	strFunc := `
                function postCredential() {
                        var textboxes = document.getElementsByName('text_box');
			sendJson = '{ "ProviderName" : "$$PROVIDER$$", "KeyValueInfoList" : $$CREDENTIALINFO$$, "CredentialName" : "$$NAME$$" }'

                        for (var i = 0; i < textboxes.length; i++) { // @todo make parallel executions
                                switch (textboxes[i].id) {
                                        case "1":
                                                sendJson = sendJson.replace("$$PROVIDER$$", textboxes[i].value);
                                                break;
                                        case "2":
                                                sendJson = sendJson.replace("$$CREDENTIALINFO$$", textboxes[i].value);
                                                break;
                                        case "3":
                                                sendJson = sendJson.replace("$$NAME$$", textboxes[i].value);
                                                break;
                                        default:
                                                break;
                                }
                        }
                        var xhr = new XMLHttpRequest();
                        xhr.open("POST", "$$TUMBLEBUG_SERVER$$/spider/credential", true);
                        xhr.setRequestHeader('Content-Type', 'application/json');
                        //xhr.send(JSON.stringify({ "CredentialName": credentialName, "ProviderName": providerName, "KeyValueInfoList": credentialInfo}));
                        //xhr.send(JSON.stringify(sendJson));
                        xhr.send(sendJson);

                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

// make the string of javascript function
func makeDeleteCredentialFunc_js() string {
	// curl -X DELETE http://$RESTSERVER:1323/spider/credential/aws-credential01 -H 'Content-Type: application/json'

	strFunc := `
                function deleteCredential() {
                        var checkboxes = document.getElementsByName('check_box');
                        for (var i = 0; i < checkboxes.length; i++) { // @todo make parallel executions
                                if (checkboxes[i].checked) {
                                        var xhr = new XMLHttpRequest();
                                        xhr.open("DELETE", "$$TUMBLEBUG_SERVER$$/spider/credential/" + checkboxes[i].value, true);
                                        xhr.setRequestHeader('Content-Type', 'application/json');
                                        xhr.send(null);
                                }
                        }
                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

//================ Credential Info Management
// create credential page
func Credential(c echo.Context) error {
	common.CBLog.Info("call credential()")

	// make page header
	htmlStr := `
                <html>
                <head>
                    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
                    <script type="text/javascript">
                `
	// (1) make Javascript Function
	htmlStr += makeOnchangeCredentialProviderFunc_js()
	htmlStr += makeCheckBoxToggleFunc_js()
	htmlStr += makePostCredentialFunc_js()
	htmlStr += makeDeleteCredentialFunc_js()

	htmlStr += `
                    </script>
                </head>

                <body>
                    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">
                `

	// (2) make Table Action TR
	// colspan, f5_href, delete_href, fontSize
	htmlStr += makeActionTR_html("5", "credential", "deleteCredential()", "2")

	// (3) make Table Header TR
	nameWidthList := []NameWidth{
		{"Provider Name", "200"},
		{"Credential Info", "300"},
		{"Credential Name", "200"},
	}
	htmlStr += makeTitleTRList_html("#DDDDDD", "2", nameWidthList)

	// (4) make Table info list TR
	// (4-1) get driver info list @todo if empty list
	resBody, err := getResourceList_JsonByte("credential")
	if err != nil {
		common.CBLog.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var info struct {
		ResultList []*cim.CredentialInfo `json:"credential"`
	}
	json.Unmarshal(resBody, &info)

	// (4-2) make Table info list TR
	htmlStr += makeCredentialTRList_html("", "", "", info.ResultList)

	// (5) make input field and add
	// attach text box for add
	htmlStr += `
                        <tr bgcolor="#FFFFFF" align="center" height="30">
                            <td>
                                    <font size=2>#</font>
                            </td>
                            <td>
				<!-- <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="1" value="AWS"> -->
		`
	// Select format of CloudOS  name=text_box, id=1
	htmlStr += makeSelect_html("onchangeProvider")

	htmlStr += `	
                            </td>
                            <td>
                                <textarea style="font-size:12px;text-align:center;" name="text_box" id="2" cols=50>[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]</textarea>
                            </td>
                            <td>
                                <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="3" value="aws-credential01">
                            </td>
                            <td>
                                <a href="javascript:postCredential()">
                                    <font size=3><b>+</b></font>
                                </a>
                            </td>
                        </tr>
                `
	// make page tail
	htmlStr += `
                    </table>
                </body>
                </html>
        `

	//fmt.Println(htmlStr)
	return c.HTML(http.StatusOK, htmlStr)
}

// make the string of javascript function
func makeOnchangeRegionProviderFunc_js() string {
	strFunc := `
              function onchangeProvider(source) {
                var providerName = source.value
        // for region info
        switch(providerName) {
          case "AWS":
            regionInfo = '[{"Key":"Region", "Value":"us-east-2"}]'
            region = '(ohio)us-east-2'
            zone = ''
            break;
          case "AZURE":
            regionInfo = '[{"Key":"location", "Value":"northeurope"}, {"Key":"ResourceGroup", "Value":"CB-GROUP-POWERKIM"}]'
            region = 'northeurope'
            zone = ''            
            break;
          case "GCP":
            regionInfo = '[{"Key":"Region", "Value":"us-central1"},{"Key":"Zone", "Value":"us-central1-a"}]'
            region = 'us-central1'
            zone = 'us-central1-a'             
            break;
          case "ALIBABA":
            regionInfo = '[{"Key":"Region", "Value":"ap-northeast-1"}, {"Key":"Zone", "Value":"ap-northeast-1a"}]'
            region = 'ap-northeast-1'
            zone = 'ap-northeast-1a'             
            break;
          case "CLOUDIT":
            regionInfo = '[{"Key":"Region", "Value":"default"}]'
            region = 'default'
            zone = ''            
            break;
          case "OPENSTACK":
            regionInfo = '[{"Key":"Region", "Value":"RegionOne"}]'
            region = 'RegionOne'
            zone = 'RegionOne'            
            break;
          case "DOCKER":
            regionInfo = '[{"Key":"Region", "Value":"default"}]'
            region = 'default'
            zone = ''             
            break;
          case "CLOUDTWIN":
            regionInfo = '[{"Key":"Region", "Value":"default"}]'
            region = 'default'
            zone = '' 
            break;
          default:
            regionInfo = '[{"Key":"Region", "Value":"us-east-2"}]'
            region = '(ohio)us-east-2'
            zone = ''
        }
                document.getElementById('2').value= regionInfo

        // for region-zone name
                document.getElementById('3').value= providerName.toLowerCase() + "-" + region + "-" + zone;
              }
        `
	return strFunc
}

// number, Provider Name, Region Info, Region Name, checkbox
func makeRegionTRList_html(bgcolor string, height string, fontSize string, infoList []*rim.RegionInfo) string {
	if bgcolor == "" {
		bgcolor = "#FFFFFF"
	}
	if height == "" {
		height = "30"
	}
	if fontSize == "" {
		fontSize = "2"
	}

	// make base TR frame for info list
	strTR := fmt.Sprintf(`
                <tr bgcolor="%s" align="center" height="%s">
                    <td>
                            <font size=%s>$$NUM$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S1$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S2$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S3$$</font>
                    </td>
                    <td>
                        <input type="checkbox" name="check_box" value=$$S3$$>
                    </td>
                </tr>
                `, bgcolor, height, fontSize, fontSize, fontSize, fontSize)

	strData := ""
	// set data and make TR list
	for i, one := range infoList {
		str := strings.ReplaceAll(strTR, "$$NUM$$", strconv.Itoa(i+1))
		str = strings.ReplaceAll(str, "$$S1$$", one.ProviderName)
		strKeyList := ""
		for _, kv := range one.KeyValueInfoList {
			strKeyList += kv.Key + ":" + kv.Value + ", "
		}
		str = strings.ReplaceAll(str, "$$S2$$", strKeyList)
		str = strings.ReplaceAll(str, "$$S3$$", one.RegionName)
		strData += str
	}

	return strData
}

// make the string of javascript function
func makePostRegionFunc_js() string {

	// curl -X POST http://$RESTSERVER:1323/spider/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-(ohio)us-east-2","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"us-east-2"}]}'

	strFunc := `
                function postRegion() {
                        var textboxes = document.getElementsByName('text_box');
            sendJson = '{ "ProviderName" : "$$PROVIDER$$", "KeyValueInfoList" : $$REGIONINFO$$, "RegionName" : "$$NAME$$" }'

                        for (var i = 0; i < textboxes.length; i++) { // @todo make parallel executions
                                switch (textboxes[i].id) {
                                        case "1":
                                                sendJson = sendJson.replace("$$PROVIDER$$", textboxes[i].value);
                                                break;
                                        case "2":
                                                sendJson = sendJson.replace("$$REGIONINFO$$", textboxes[i].value);
                                                break;
                                        case "3":
                                                sendJson = sendJson.replace("$$NAME$$", textboxes[i].value);
                                                break;
                                        default:
                                                break;
                                }
                        }
                        var xhr = new XMLHttpRequest();
                        xhr.open("POST", "$$TUMBLEBUG_SERVER$$/spider/region", true);
                        xhr.setRequestHeader('Content-Type', 'application/json');
                        //xhr.send(JSON.stringify({ "RegionName": regionName, "ProviderName": providerName, "KeyValueInfoList": regionInfo}));
                        //xhr.send(JSON.stringify(sendJson));
                        xhr.send(sendJson);

                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

// make the string of javascript function
func makeDeleteRegionFunc_js() string {
	// curl -X DELETE http://$RESTSERVER:1323/spider/region/aws-(ohio)us-east-2 -H 'Content-Type: application/json'

	strFunc := `
                function deleteRegion() {
                        var checkboxes = document.getElementsByName('check_box');
                        for (var i = 0; i < checkboxes.length; i++) { // @todo make parallel executions
                                if (checkboxes[i].checked) {
                                        var xhr = new XMLHttpRequest();
                                        xhr.open("DELETE", "$$TUMBLEBUG_SERVER$$/spider/region/" + checkboxes[i].value, true);
                                        xhr.setRequestHeader('Content-Type', 'application/json');
                                        xhr.send(null);
                                }
                        }
                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

//================ Region Info Management
// create region page
func Region(c echo.Context) error {
	common.CBLog.Info("call region()")

	// make page header
	htmlStr := `
                <html>
                <head>
                    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
                    <script type="text/javascript">
                `
	// (1) make Javascript Function
	htmlStr += makeOnchangeRegionProviderFunc_js()
	htmlStr += makeCheckBoxToggleFunc_js()
	htmlStr += makePostRegionFunc_js()
	htmlStr += makeDeleteRegionFunc_js()

	htmlStr += `
                    </script>
                </head>

                <body>
                    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">
                `

	// (2) make Table Action TR
	// colspan, f5_href, delete_href, fontSize
	htmlStr += makeActionTR_html("5", "region", "deleteRegion()", "2")

	// (3) make Table Header TR
	nameWidthList := []NameWidth{
		{"Provider Name", "200"},
		{"Region Info", "300"},
		{"Region Name", "200"},
	}
	htmlStr += makeTitleTRList_html("#DDDDDD", "2", nameWidthList)

	// (4) make Table info list TR
	// (4-1) get driver info list @todo if empty list
	resBody, err := getResourceList_JsonByte("region")
	if err != nil {
		common.CBLog.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var info struct {
		ResultList []*rim.RegionInfo `json:"region"`
	}
	json.Unmarshal(resBody, &info)

	// (4-2) make Table info list TR
	htmlStr += makeRegionTRList_html("", "", "", info.ResultList)

	// (5) make input field and add
	// attach text box for add
	htmlStr += `
                        <tr bgcolor="#FFFFFF" align="center" height="30">
                            <td>
                                    <font size=2>#</font>
                            </td>
                            <td>
                <!-- <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="1" value="AWS"> -->
        `
	// Select format of CloudOS  name=text_box, id=1
	htmlStr += makeSelect_html("onchangeProvider")

	htmlStr += `    
                            </td>
                            <td>
                                <textarea style="font-size:12px;text-align:center;" name="text_box" id="2" cols=50>[{"Key":"Region", "Value":"us-east-2"}]</textarea>
                            </td>
                            <td>
                                <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="3" value="aws-(ohio)us-east-2">
                            </td>
                            <td>
                                <a href="javascript:postRegion()">
                                    <font size=3><b>+</b></font>
                                </a>
                            </td>
                        </tr>
                `
	// make page tail
	htmlStr += `
                    </table>
                </body>
                </html>
        `

	//fmt.Println(htmlStr)
	return c.HTML(http.StatusOK, htmlStr)
}

// make the string of javascript function
func makeOnchangeConnectionConfigProviderFunc_js() string {
	strFunc := `
              function onchangeProvider(source) {
                var providerName = source.value
        // for credential info
        switch(providerName) {
          case "AWS":
            credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
            break;
          case "AZURE":
            credentialInfo = '[{"Key":"ClientId", "Value":"XXXX-XXXX"}, {"Key":"ClientSecret", "Value":"xxxx-xxxx"}, {"Key":"TenantId", "Value":"xxxx-xxxx"}, {"Key":"SubscriptionId", "Value":"xxxx-xxxx"}]'
            break;
          case "GCP":
            credentialInfo = '[{"Key":"PrivateKey", "Value":"-----BEGIN PRIVATE KEY-----\nXXXX\n-----END PRIVATE KEY-----\n"},{"Key":"ProjectID", "Value":"powerkimhub"}, {"Key":"ClientEmail", "Value":"xxxx@xxxx.iam.gserviceaccount.com"}]'
            break;
          case "ALIBABA":
            credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
            break;
          case "CLOUDIT":
            credentialInfo = '[{"Key":"IdentityEndpoint", "Value":"http://xxx.xxx.co.kr:9090"}, {"Key":"AuthToken", "Value":"xxxx"}, {"Key":"Username", "Value":"xxxx"}, {"Key":"Password", "Value":"xxxx"}, {"Key":"TenantId", "Value":"tnt0009"}]'
            break;
          case "OPENSTACK":
            credentialInfo = '[{"Key":"IdentityEndpoint", "Value":"http://182.252.xxx.xxx:5000/v3"}, {"Key":"Username", "Value":"etri"}, {"Key":"Password", "Value":"xxxx"}, {"Key":"DomainName", "Value":"default"}, {"Key":"ProjectID", "Value":"xxxx"}]'
            break;
          case "DOCKER":
            credentialInfo = '[{"Key":"Host", "Value":"http://18.191.xxx.xxx:1004"}, {"Key":"APIVersion", "Value":"v1.38"}]'
            break;
          case "CLOUDTWIN":
            credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
            break;
          default:
            credentialInfo = '[{"Key":"ClientId", "Value":"XXXXXX"}, {"Key":"ClientSecret", "Value":"XXXXXX"}]'
        }
                document.getElementById('2').value= credentialInfo

        // for credential name
                document.getElementById('3').value= providerName.toLowerCase() + "-credential-01";
              }
        `
	return strFunc
}

// number, Provider Name, Driver Name, Credential Name, Region Name, Connection Name, checkbox
func makeConnectionConfigTRList_html(bgcolor string, height string, fontSize string, infoList []*ccim.ConnectionConfigInfo) string {
	if bgcolor == "" {
		bgcolor = "#FFFFFF"
	}
	if height == "" {
		height = "30"
	}
	if fontSize == "" {
		fontSize = "2"
	}

	// make base TR frame for info list
	strTR := fmt.Sprintf(`
                <tr bgcolor="%s" align="center" height="%s">
                    <td>
                            <font size=%s>$$NUM$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S1$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S2$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S3$$</font>
                    </td>
                    <td>
                            <font size=%s>$$S4$$</font>
                    </td>
                                                            <td>
                            <font size=%s>$$S5$$</font>
                    </td>
                    <td>
                        <input type="checkbox" name="check_box" value=$$S3$$>
                    </td>
                </tr>
                `, bgcolor, height, fontSize, fontSize, fontSize, fontSize)

	strData := ""
	// set data and make TR list
	for i, one := range infoList {
		str := strings.ReplaceAll(strTR, "$$NUM$$", strconv.Itoa(i+1))
		str = strings.ReplaceAll(str, "$$S1$$", one.ProviderName)
		str = strings.ReplaceAll(str, "$$S2$$", one.DriverName)
		str = strings.ReplaceAll(str, "$$S3$$", one.CredentialName)
		str = strings.ReplaceAll(str, "$$S4$$", one.RegionName)
		str = strings.ReplaceAll(str, "$$S5$$", one.ConfigName)
		strData += str
	}

	return strData
}

// make the string of javascript function
func makePostConnectionConfigFunc_js() string {

	// curl -X POST http://$RESTSERVER:1323/spider/connectionconfig -H 'Content-Type: application/json'
	//    -d '{"ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-ohio", "ConfigName":"aws-ohio-config",}'

	strFunc := `
                function postConnection() {
                        var textboxes = document.getElementsByName('text_box');
            sendJson = '{ "ProviderName" : "$$PROVIDER$$", "DriverName" : $$DRIVERNAME$$, "CredentialName" : "$$CREDENTIALNAME$$", 
                                                "RegionName" : "$$REGIONNAME$$", "ConfigName" : "$$NAME$$" }'

                        for (var i = 0; i < textboxes.length; i++) { // @todo make parallel executions
                                switch (textboxes[i].id) {
                                        case "1":
                                                sendJson = sendJson.replace("$$PROVIDER$$", textboxes[i].value);
                                                break;
                                        case "2":
                                                sendJson = sendJson.replace("$$DRIVERNAME$$", textboxes[i].value);
                                                break;
                                        case "3":
                                                sendJson = sendJson.replace("$$CREDENTIALNAME$", textboxes[i].value);
                                                break;
                                        case "4":
                                                sendJson = sendJson.replace("$$REGIONNAME$$", textboxes[i].value);
                                                break;                                                
                                        case "5":
                                                sendJson = sendJson.replace("$$NAME$$", textboxes[i].value);
                                                break;
                                        default:
                                                break;
                                }
                        }
                        var xhr = new XMLHttpRequest();
                        xhr.open("POST", "$$TUMBLEBUG_SERVER$$/spider/connection", true);
                        xhr.setRequestHeader('Content-Type', 'application/json');
                        xhr.send(sendJson);

                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

// make the string of javascript function
func makeDeleteConnectionConfigFunc_js() string {
	// curl -X DELETE http://$RESTSERVER:1323/spider/connectionconfig/aws-connection01 -H 'Content-Type: application/json'

	strFunc := `
                function deleteConnection() {
                        var checkboxes = document.getElementsByName('check_box');
                        for (var i = 0; i < checkboxes.length; i++) { // @todo make parallel executions
                                if (checkboxes[i].checked) {
                                        var xhr = new XMLHttpRequest();
                                        xhr.open("DELETE", "$$TUMBLEBUG_SERVER$$/spider/connectionconfig/" + checkboxes[i].value, true);
                                        xhr.setRequestHeader('Content-Type', 'application/json');
                                        xhr.send(null);
                                }
                        }
                        setTimeout(function(){
                                location.reload();
                        }, 400);

                }
        `
	strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

//================ Connection Config Info Management
// create Connection page
func Connectionconfig(c echo.Context) error {
	common.CBLog.Info("call connectionconfig()")

	// make page header
	htmlStr := `
                <html>
                <head>
                    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
                    <script type="text/javascript">
                `
	// (1) make Javascript Function
	htmlStr += makeOnchangeConnectionConfigProviderFunc_js()
	htmlStr += makeCheckBoxToggleFunc_js()
	htmlStr += makePostConnectionConfigFunc_js()
	htmlStr += makeDeleteConnectionConfigFunc_js()

	htmlStr += `
                    </script>
                </head>

                <body>
                    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">
                `

	// (2) make Table Action TR
	// colspan, f5_href, delete_href, fontSize
	htmlStr += makeActionTR_html("5", "connectionconfig", "deleteConnectionConfig()", "2")

	// (3) make Table Header TR
	nameWidthList := []NameWidth{
		{"Provider Name", "200"},
		{"Driver Name", "200"},
		{"Credential Name", "200"},
		{"Region Name", "200"},
		{"Connection Config Name", "200"},
	}
	htmlStr += makeTitleTRList_html("#DDDDDD", "2", nameWidthList)

	// (4) make Table info list TR
	// (4-1) get driver info list @todo if empty list
	resBody, err := getResourceList_JsonByte("connectionconfig")
	if err != nil {
		common.CBLog.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var info struct {
		ResultList []*ccim.ConnectionConfigInfo `json:"connectionconfig"`
	}
	json.Unmarshal(resBody, &info)

	// (4-2) make Table info list TR
	htmlStr += makeConnectionConfigTRList_html("", "", "", info.ResultList)

	// (5) make input field and add
	// attach text box for add
	htmlStr += `
                        <tr bgcolor="#FFFFFF" align="center" height="30">
                            <td>
                                    <font size=2>#</font>
                            </td>
                            <td>
                <!-- <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="1" value="AWS"> -->
        `
	// Select format of CloudOS  name=text_box, id=1
	htmlStr += makeSelect_html("onchangeProvider")

	htmlStr += `    
                            </td>
                            <td>
                                <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="2" value="aws-driver-v1.0">
                            </td>
                            <td>
                                <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="3" value="aws-credential01">
                            </td>
                            <td>
                                <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="4" value="aws-region01">
                            </td>
                            <td>
                                <input style="font-size:12px;text-align:center;" type="text" name="text_box" id="5" value="aws-connection-config01">
                            </td>

                            <td>
                                <a href="javascript:postConnectionConfig()">
                                    <font size=3><b>+</b></font>
                                </a>
                            </td>
                        </tr>
                `
	// make page tail
	htmlStr += `
                    </table>
                </body>
                </html>
        `

	//fmt.Println(htmlStr)
	return c.HTML(http.StatusOK, htmlStr)
}

//================ This Tumblebug Info
func SpiderInfo(c echo.Context) error {
	common.CBLog.Info("call spiderInfo()")

	htmlStr := `
                <html>
                <head>
                    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
                </head>

                <body>

                    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">
                                <tr bgcolor="#DDDDDD" align="center">
                                    <td width="200">
                                            <font size=2>Server Start Time</font>
                                    </td>
                                    <td width="220">
                                            <font size=2>Server Version</font>
                                    </td>
                                    <td width="220">
                                            <font size=2>API Version</font>
                                    </td>
                                </tr>
                                <tr bgcolor="#FFFFFF" align="center" height="30">
                                    <td width="220">
                                            <font size=2>$$STARTTIME$$</font>
                                    </td>
                                    <td width="220">
                                            <font size=2>CB-Tumblebug v0.2.0 (Cappuccino)</font>
                                    </td>
                                    <td width="220">
                                            <font size=2>REST API v0.2.0 (Cappuccino)</font>
                                    </td>
                                </tr>

                    </table>
		<br>
		<br>
		<br>
                    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">
                                <tr bgcolor="#DDDDDD" align="center">
                                    <td width="240">
                                            <font size=2>API EndPoint</font>
                                    </td>
                                    <td width="420">
                                            <font size=2>API Docs</font>
                                    </td>
                                </tr>
                                <tr bgcolor="#FFFFFF" align="left" height="30">
                                    <td width="240">
                                            <font size=2>$$APIENDPOINT$$</font>
                                    </td>
                                    <td width="420">
                                            <font size=2>
                                            * Cloud Connection Info Mgmt:
                                                <br>
                                                &nbsp;&nbsp;&nbsp;&nbsp;<a href='https://cloud-barista.github.io/rest-api/v0.2.0/spider/ccim/' target='_blank'>
                                                    https://cloud-barista.github.io/rest-api/v0.2.0/spider/ccim/
                                                </a>
                                            </font>
                                            <p>
                                            <font size=2>
                                                * Cloud Resource Control Mgmt: 
                                                <br>
                                                &nbsp;&nbsp;&nbsp;&nbsp;<a href='https://cloud-barista.github.io/rest-api/v0.2.0/spider/cctm/' target='_blank'>
                                                    https://cloud-barista.github.io/rest-api/v0.2.0/spider/cctm/
                                                </a>
                                            </font>
                                    </td>
                                </tr>

                    </table>
                </body>
                </html>
                `

	htmlStr = strings.ReplaceAll(htmlStr, "$$STARTTIME$$", cr.StartTime)
	htmlStr = strings.ReplaceAll(htmlStr, "$$APIENDPOINT$$", "http://"+cr.HostIPorName+cr.ServicePort+"/spider") // cr.ServicePort = ":1323"

	return c.HTML(http.StatusOK, htmlStr)
}
