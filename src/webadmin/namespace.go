package webadmin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/labstack/echo/v4"
)

func makePostNsFunc_js() string {

	// curl -X POST http://$RESTSERVER:1323/tumblebug/ns -H 'Content-Type: application/json'  -d '{"name": "webadmin-test", "description": "webadmin-test-desc"}'

	strFunc := `
        function postNs() {
            var textboxes = document.getElementsByName('text_box');
			sendJson = '{ "name": "$$NS_NAME$$", "description": "$$NS_DESC$$" }'
            for (var i = 0; i < textboxes.length; i++) { // @todo make parallel executions
                switch (textboxes[i].id) {
                    case "2":
                        sendJson = sendJson.replace("$$NS_NAME$$", textboxes[i].value);
                        break;
                    case "3":
                        sendJson = sendJson.replace("$$NS_DESC$$", textboxes[i].value);
                        break;
                    default:
                        break;
                }
            }
            var xhr = new XMLHttpRequest();
            //xhr.open("POST", "$$TUMBLEBUG_SERVER$$/tumblebug/ns", true);
            xhr.open("POST", "/tumblebug/ns", true);
            xhr.setRequestHeader('Content-Type', 'application/json');
            xhr.send(sendJson);

            setTimeout(function(){
				window.top.location.reload();
            }, 400);

        }
        `
	//strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

func makeDeleteNsFunc_js() string {
	// curl -X DELETE http://$RESTSERVER:1323/tumblebug/ns/webadmin-test -H 'Content-Type: application/json'

	strFunc := `
		function deleteNs() {
			var checkboxes = document.getElementsByName('check_box');
			for (var i = 0; i < checkboxes.length; i++) { // @todo make parallel executions
				if (checkboxes[i].checked) {
					var xhr = new XMLHttpRequest();
					//xhr.open("DELETE", "$$TUMBLEBUG_SERVER$$/tumblebug/ns/" + checkboxes[i].value, true);
					xhr.open("DELETE", "/tumblebug/ns/" + checkboxes[i].value, true);
					//xhr.setRequestHeader('Content-Type', 'application/json');
					xhr.send(null);
				}
			}
			setTimeout(function(){
				window.top.location.reload();
			}, 400);

		}
        `
	//strFunc = strings.ReplaceAll(strFunc, "$$TUMBLEBUG_SERVER$$", "http://"+cr.HostIPorName+cr.ServicePort) // cr.ServicePort = ":1323"
	return strFunc
}

func makeNsTRList_html(bgcolor string, height string, fontSize string, infoList []common.NsInfo) string {
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
                        <input type="checkbox" name="check_box" value=$$S1$$>
                    </td>
                </tr>
       		`, bgcolor, height, fontSize, fontSize, fontSize, fontSize)

	strData := ""
	// set data and make TR list
	for i, one := range infoList {
		str := strings.ReplaceAll(strTR, "$$NUM$$", strconv.Itoa(i+1))
		str = strings.ReplaceAll(str, "$$S1$$", one.Id)
		str = strings.ReplaceAll(str, "$$S2$$", one.Name)
		str = strings.ReplaceAll(str, "$$S3$$", one.Description)
		strData += str
	}

	return strData
}

func listNsId() []string {
	resBody, err := getTbResourceList_JsonByte("ns")
	if err != nil {
		common.CBLog.Error(err)
	}
	var info struct {
		ResultList []common.NsInfo `json:"ns"`
	}
	json.Unmarshal(resBody, &info)

	//return info.ResultList
	var res []string
	for _, v := range info.ResultList {
		res = append(res, v.Id)
	}
	return res
}

func makeNsSelector_html(onchangeFunctionName string) string {
	strList := listNsId()

	strSelect := `<select name="text_box" id="1" onchange="` + onchangeFunctionName + `(this)">`
	for _, one := range strList {

		strSelect += `<option value="` + one + `">` + one + `</option>`

	}

	strSelect += `
		</select>
	`

	return strSelect
}

func Ns(c echo.Context) error {
	common.CBLog.Info("call Ns()")

	// make page header
	htmlStr := ` 
		<html>
		<head>
		    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
		    <script type="text/javascript">
		`
	// (1) make Javascript Function
	//htmlStr += makeOnchangeDriverProviderFunc_js()
	htmlStr += makeCheckBoxToggleFunc_js()
	htmlStr += makePostNsFunc_js()
	htmlStr += makeDeleteNsFunc_js()

	htmlStr += `
		    </script>
		</head>

		<body>
		    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">      
		`

	// (2) make Table Action TR
	// colspan, f5_href, delete_href, fontSize
	htmlStr += makeActionTR_html("5", "ns", "deleteNs()", "2")

	// (3) make Table Header TR

	nameWidthList := []NameWidth{
		{"Namespace Id", "300"},
		{"Namespace Name", "300"},
		{"Description", "300"},
	}
	htmlStr += makeTitleTRList_html("#DDDDDD", "2", nameWidthList)

	// (4) make Table info list TR
	// (4-1) get driver info list @todo if empty list
	resBody, err := getTbResourceList_JsonByte("ns")
	if err != nil {
		common.CBLog.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var info struct {
		ResultList []common.NsInfo `json:"ns"`
	}
	json.Unmarshal(resBody, &info)

	// (4-2) make Table info list TR
	htmlStr += makeNsTRList_html("", "", "", info.ResultList)

	// (5) make input field and add
	// attach text box for add
	htmlStr += `
			<tr bgcolor="#FFFFFF" align="center" height="30">
			    <td>
				    <font size=2>#</font>
                </td>
                <td>
                    
                </td>
			    <td>
				<input style="font-size:12px;text-align:center;" type="text" name="text_box" id="2" value="webadmin-test">
		`
	// Select format of CloudOS  name=text_box, id=1
	//htmlStr += makeSelect_html("onchangeProvider")

	htmlStr += `
			    </td>
			    <td>
				<input style="font-size:12px;text-align:center;" type="text" name="text_box" id="3" value="webadmin-test-desc">
			    </td>
			    <td>
				<a href="javascript:postNs()">
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
