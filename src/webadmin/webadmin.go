package webadmin

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo"
	//cr "github.com/cloud-barista/cb-spider/api-runtime/common-runtime"
	"github.com/cloud-barista/cb-tumblebug/src/common"
)

//var cblog *logrus.Logger

func init() {
	//cblog = config.Cblogger
}

type NameWidth struct {
	Name  string
	Width string
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
    <frameset cols="200,*" frameborder="Yes" border=1">
        <frame src="webadmin/menu" name="top_frame" scrolling="auto" noresize marginwidth="0" marginheight="0"/>
        <frameset frameborder="Yes" border=1">
            <frame src="webadmin/ns" name="main_frame" scrolling="auto" noresize marginwidth="5" marginheight="0"/> 
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
	common.CBLog.Info("call Menu()")

	htmlStr := ` 
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
</head>
<body>
    <br>
    <!-- <table border="0" bordercolordark="#FFFFFF" cellpadding="0" cellspacing="2" bgcolor="#FFFFFF" width="320" style="font-size:small;"> -->
    <table border="1" bordercolordark="#FFFFFF" cellpadding="10" cellspacing="1" bgcolor="#FFFFFF"  style="font-size:small;">      
        <tr bgcolor="#FFFFFF" align="center">
            <td width="100" bgcolor="#FFFFFF">
                <!-- CB-Tumblebug Logo -->
                <a href="../webadmin" target="_top">
                  <img height="45" width="42" src="https://cloud-barista.github.io/assets/img/frameworks/cb-tumblebug.png" border='0' hspace='0' vspace='1' align="middle">
                </a>
            </td>
        </tr>
        <tr>
            <td width="200">       
                <a href="tumblebugInfo" target="main_frame">            
                    <!--<font size=2>CB-Tumblebug info</font>-->
                </a><br><br><br>
            </td>
        </tr>
        <tr>
            <td width="100">       
                <a href="ns" target="main_frame">            
                    <font size=2>Namespace</font>
                </a><br><br><br>
            </td>
        </tr>
        <tr>
            <td width="100">
            `

	htmlStr += makeNsSelector_html("onchangeNs")

	htmlStr += `
            </td>
        </tr>
        <tr>
            <td width="100">       
            <!--<font size=2>[MCIR]</font>-->
            </td>
        </tr>
        <tr>
            <td width="100">       
                <a href="image" target="main_frame">            
                <!--<font size=2>Image</font>-->
                </a>
            </td>
        </tr>
        <tr>
            <td width="100">       
                <a href="spec" target="main_frame">            
                <!--<font size=2>Spec</font>-->
                </a>
            </td>
        </tr>
        <tr>
            <td width="100">
                <a href="vNet" target="main_frame">
                <!--<font size=2>vNet + Subnet</font>-->
                </a>
            </td>
        </tr>
        <tr>
            <td width="100">
                <a href="securityGroup" target="_blank">
                <!--<font size=2>Security Group</font>-->
                </a>
            </td>
        </tr>
        <tr>
            <td width="100">
                <a href="sshKey" target="_blank">
                <!--<font size=2>SSH Key</font>-->
                </a><br><br><br>
            </td>
        </tr>
        <tr>
            <td width="100">       
            <!--<font size=2>[MCIS]</font>-->
            </td>
        </tr>
        <tr>
            <td width="100">
                <a href="mcis" target="_blank">
                <!--<font size=2>MCIS</font>-->
                </a><br><br><br>
            </td>            
        </tr>
        <tr>
            <td width="100">       
                <a href="https://github.com/cloud-barista/cb-tumblebug" target="_blank">            
                    <font size=2>GitHub</font>
                </a>
            </td> 
        </tr>
    </table>
</body>
</html>
	`

	//htmlStr = strings.ReplaceAll(htmlStr, "$$TIME$$", cr.ShortStartTime)
	return c.HTML(http.StatusOK, htmlStr)
}

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

func makeActionTR_html(colspan string, f5_href string, delete_href string, fontSize string) string {
	if fontSize == "" {
		fontSize = "2"
	}

	strTR := fmt.Sprintf(`
		<tr bgcolor="#FFFFFF" align="right">
		    <td colspan="%s">
			<a href="%s">
			    <font size=%s><b>&nbsp;Refresh</b></font>
			</a>
			&nbsp;
			<a href="javascript:%s;">
			    <font size=%s><b>&nbsp;Delete selected</b></font>
			</a>
			&nbsp;
		    </td>
		</tr>
       		`, colspan, f5_href, fontSize, delete_href, fontSize)

	return strTR
}

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

func getTbResourceList_JsonByte(resourceName string) ([]byte, error) {
	//fmt.Println("getTbResourceList_JsonByte(" + resourceName + ") called") // for debug
	// cr.ServicePort = ":1323"
	url := "http://localhost:1323/tumblebug/" + resourceName

	// get object list
	res, err := http.Get(url)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	resBody, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	fmt.Println("resBody: " + string(resBody)) // for debug
	return resBody, err
}

func getSpiderResourceList_JsonByte(resourceName string) ([]byte, error) {
	//fmt.Println("getTbResourceList_JsonByte(" + resourceName + ") called") // for debug
	// cr.ServicePort = ":1323"
	url := "http://localhost:1024/spider/" + resourceName

	// get object list
	res, err := http.Get(url)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	resBody, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	fmt.Println("resBody: " + string(resBody)) // for debug
	return resBody, err
}

/*
func makeConnectionConfigSelector_html(onchangeFunctionName string) string {
	strList := listConnectionName()

	strSelect := `<select name="text_box" id="1" onchange="` + onchangeFunctionName + `(this)">`
	for _, one := range strList {

		strSelect += `<option value="` + one + `">` + one + `</option>`

	}

	strSelect += `
		</select>
	`

	return strSelect
}
*/
