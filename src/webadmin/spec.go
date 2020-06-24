package webadmin

import (
	"encoding/json"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/labstack/echo"
)

func makePostSpecFunc_js() string {

	// curl -X POST http://$RESTSERVER:1323/tumblebug/ns/{ns_id}/resources/spec -H 'Content-Type: application/json'  -d '{"name": "webadmin-test", "description": "webadmin-test-desc"}'

	strFunc := `
        function postNs() {
            var textboxes = document.getElementsByName('text_box');
			sendJson = '{
				"name": "$$SPEC_NAME$$",
				"cspSpecName": "$$CSPSPECNAME",
				"connectionName": "$$CONNECTION_NAME$$",
				"os_type": "",
				"num_vCPU": "$$num_vCPU$$",
				"num_core": "$$num_core$$",
				"mem_GiB": "$$mem_GiB$$",
				"storage_GiB": "",
				"description": "",
				"cost_per_hour": "",
				"num_storage": "",
				"max_num_storage": "",
				"max_total_storage_TiB": "",
				"net_bw_Gbps": "",
				"ebs_bw_Mbps": "",
				"gpu_model": "",
				"num_gpu": "",
				"gpumem_GiB": "",
				"gpu_p2p": ""
			}'
            for (var i = 0; i < textboxes.length; i++) { // @todo make parallel executions
                switch (textboxes[i].id) {
					case "2":
                        sendJson = sendJson.replace("$$SPEC_NAME$$", textboxes[i].value);
                        break;
                    case "3":
                        sendJson = sendJson.replace("$$CONNECTION_NAME$$", textboxes[i].value);
						break;
					case "4":
						sendJson = sendJson.replace("$$CSPSPECNAME$$", textboxes[i].value);
						break;	
					case "5":
                        sendJson = sendJson.replace("$$num_vCPU$$", textboxes[i].value);
                        break;
                    case "6":
                        sendJson = sendJson.replace("$$num_core$$", textboxes[i].value);
                        break;
                    case "7":
						sendJson = sendJson.replace("$$mem_GiB$$", textboxes[i].value);
						break;	
					default:
                        break;
                }
            }
            var xhr = new XMLHttpRequest();
            //xhr.open("POST", "$$TUMBLEBUG_SERVER$$/tumblebug/ns", true);
            //xhr.open("POST", "/tumblebug/ns", true);
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

func makeDeleteSpecFunc_js() string {
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

func Spec(c echo.Context) error {
	cblog.Info("call Spec()")

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
	htmlStr += makePostSpecFunc_js()
	htmlStr += makeDeleteSpecFunc_js()

	htmlStr += `
		    </script>
		</head>

		<body>
		    <table border="0" bordercolordark="#F8F8FF" cellpadding="0" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">      
		`

	// (2) make Table Action TR
	// colspan, f5_href, delete_href, fontSize
	htmlStr += makeActionTR_html("5", "spec", "deleteSpec()", "2")

	// (3) make Table Header TR

	nameWidthList := []NameWidth{
		{"Id", "80"},
		{"Name", "80"},
		{"ConnectionName", "80"},
		{"CspSpecName", "80"},
		{"vCPU_#", "30"},
		{"core_#", "30"},
		{"mem_GiB", "30"},
	}
	htmlStr += makeTitleTRList_html("#DDDDDD", "2", nameWidthList)

	// (4) make Table info list TR
	// (4-1) get driver info list @todo if empty list
	resBody, err := getTbResourceList_JsonByte("spec")
	if err != nil {
		cblog.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var info struct {
		ResultList []common.NsInfo `json:"spec"`
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
					`
	//htmlStr += makeConnectionConfigSelector_html("onchangeConnectionConfig")

	htmlStr += `
                </td>
			    <td>
				<input style="font-size:12px;text-align:center;" type="text" name="text_box" id="2" value="webadmin-test">
		`

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
