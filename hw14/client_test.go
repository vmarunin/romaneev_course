package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// код писать тут
type XMLDataRec struct {
	Id        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}
type XMLData struct {
	Row []XMLDataRec `xml:"row"`
}

const XMLDataFilePath = "./dataset.xml"

func SearchServer(w http.ResponseWriter, r *http.Request) {
	accessToken := ""
	if hList, ok := r.Header["Accesstoken"]; ok {
		if len(hList) > 0 {
			accessToken = hList[0]
		}
	}
	if accessToken == "timeout" {
		w.WriteHeader(http.StatusOK)
		time.Sleep(time.Millisecond * 1500)
		return
	}
	if accessToken == "Bad Token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if accessToken == "Internal Error" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if accessToken == "Bad Request Bad JSON" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{{`)
		return
	}
	if accessToken == "Bad Request Unknown Error" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "unknown"}`)
		return
	}
	if accessToken == "Good Request Bad JSON" {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `"some": "thing"`)
		return
	}

	order_field := r.FormValue("order_field")
	if order_field == "" {
		order_field = "Name"
	}
	if !(order_field == "Name" || order_field == "Age" || order_field == "Id") {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "ErrorBadOrderField"}`)
		return
	}
	order_by_str := r.FormValue("order_by")
	order_by := 0
	if order_by_str == "-1" {
		order_by = -1
	}
	if order_by_str == "1" {
		order_by = 1
	}
	query := r.FormValue("query")
	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "ErrorBadLimitField"}`)
		return
	}
	offset, err := strconv.Atoi(r.FormValue("offset"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "ErrorBadOffsetField"}`)
		return
	}

	file, err := os.Open(XMLDataFilePath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "Can't open XML File"}`)
		return
	}

	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "Can't read XML File"}`)
		return
	}
	var data XMLData
	err = xml.Unmarshal([]byte(fileContents), &data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "Can't parse XML File"}`)
		return
	}

	respData := make([]User, 0, len(data.Row))
	for _, rec := range data.Row {
		name := rec.FirstName + " " + rec.LastName
		if len(query) > 0 && !strings.Contains(name, query) && !strings.Contains(rec.About, query) {
			continue
		}
		respData = append(respData, User{
			Id:     rec.Id,
			Name:   name,
			Age:    rec.Age,
			Gender: rec.Gender,
			About:  rec.About,
		})
	}
	if order_by != 0 {
		if order_field == "Name" {
			sort.Slice(respData, func(i, j int) bool {
				if order_by == 1 {
					return respData[i].Name < respData[j].Name
				}
				return respData[i].Name > respData[j].Name
			})
		} else if order_field == "Id" {
			sort.Slice(respData, func(i, j int) bool {
				if order_by == 1 {
					return respData[i].Id < respData[j].Id
				}
				return respData[i].Id > respData[j].Id
			})
		} else if order_field == "Age" {
			sort.Slice(respData, func(i, j int) bool {
				if order_by == 1 {
					return respData[i].Age < respData[j].Age
				}
				return respData[i].Age > respData[j].Age
			})
		}
	}

	if offset < len(respData) {
		respData = respData[offset:]
	}
	if limit < len(respData) {
		respData = respData[:limit]
	}

	jsonData, _ := json.Marshal(respData)
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}

func TestBadSearchRequestParam(t *testing.T) {
	cases := []SearchRequest{
		{Limit: -1},
		{Offset: -1},
	}
	errMessages := []string{
		"limit must be > 0",
		"offset must be > 0",
	}

	client := new(SearchClient)

	for i := 0; i < len(cases); i++ {
		result, err := client.FindUsers(cases[i])

		if result != nil || err == nil || err.Error() != errMessages[i] {
			t.Errorf("Expected \"%s\" error, got err: %#v result: %#v", errMessages[i], err, result)
		}
	}
}

func TestBadURL(t *testing.T) {
	client := new(SearchClient)
	client.URL = "http://256.257.258.259"

	result, err := client.FindUsers(SearchRequest{})

	if result != nil || err == nil || !strings.HasPrefix(err.Error(), "unknown error") {
		t.Errorf("Expected \"unknown error\" error, got err: %#v result: %#v", err, result)
	}
}

func TestTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	client := new(SearchClient)
	client.AccessToken = "timeout"
	client.URL = ts.URL

	var req SearchRequest
	result, err := client.FindUsers(req)

	if result != nil || err == nil || !strings.HasPrefix(err.Error(), "timeout") {
		t.Errorf("Expected \"timeout\" error, got err: %#v result: %#v", err, result)
	}

	ts.Close()
}

func TestAccessToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	client := new(SearchClient)
	client.URL = ts.URL
	client.AccessToken = "Bad Token"

	var req SearchRequest
	result, err := client.FindUsers(req)

	if result != nil || err == nil || err.Error() != "Bad AccessToken" {
		t.Errorf("Expected \"Bad AccessToken\" error, got err: %#v result: %#v", err, result)
	}

	ts.Close()
}

func TestInternalServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	client := new(SearchClient)
	client.URL = ts.URL
	client.AccessToken = "Internal Error"

	var req SearchRequest
	result, err := client.FindUsers(req)

	if result != nil || err == nil || err.Error() != "SearchServer fatal error" {
		t.Errorf("Expected \"SearchServer fatal error\" error, got err: %#v result: %#v", err, result)
	}

	ts.Close()
}

func TestErrorResponseFromServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	client := new(SearchClient)
	client.URL = ts.URL
	var req SearchRequest

	tokens := []string{
		"Bad Request Bad JSON",
		"Bad Request Unknown Error",
		"Good Request Bad JSON",
	}
	errMessages := []string{
		"cant unpack error json:",
		"unknown bad request error:",
		"cant unpack result json:",
	}

	for i := 0; i < len(tokens); i++ {
		client.AccessToken = tokens[i]
		result, err := client.FindUsers(req)

		if result != nil || err == nil || !strings.HasPrefix(err.Error(), errMessages[i]) {
			t.Errorf("Expected \"%s\" error, got err: %#v result: %#v", errMessages[i], err, result)
		}
	}

	ts.Close()
}

func TestBadOrderField(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	client := new(SearchClient)
	client.URL = ts.URL

	var req SearchRequest
	req.OrderField = "bad"
	result, err := client.FindUsers(req)

	if result != nil || err == nil || !strings.HasPrefix(err.Error(), "OrderFeld") {
		t.Errorf("Expected \"OrderFeld invalid\" error, got err: %#v result: %#v", err, result)
	}

	ts.Close()
}

func TestOkResp(t *testing.T) {
	testReq := []SearchRequest{
		{Limit: 10, OrderField: "Id", OrderBy: -1, Offset: 33},
		{Limit: 1, OrderField: "Age", OrderBy: 1, Offset: 3},
		{Limit: 26, OrderField: "Name"},
	}
	testResp := []string{
		`{"Users":[
			{"Id":1,"Name":"Hilda Mayer","Age":21,"About":"Sit commodo consectetur minim amet ex. Elit aute mollit fugiat labore sint ipsum dolor cupidatat qui reprehenderit. Eu nisi in exercitation culpa sint aliqua nulla nulla proident eu. Nisi reprehenderit anim cupidatat dolor incididunt laboris mollit magna commodo ex. Cupidatat sit id aliqua amet nisi et voluptate voluptate commodo ex eiusmod et nulla velit.\n","Gender":"female"},
			{"Id":0,"Name":"Boyd Wolf","Age":22,"About":"Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n","Gender":"male"}
		],"NextPage":false}`,
		`{"Users":	[
			{"Id":0,"Name":"Boyd Wolf","Age":22,"About":"Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n","Gender":"male"}
		],"NextPage":true}`,
		`{"Users":	[
			{"Id":0,"Name":"Boyd Wolf","Age":22,"About":"Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n","Gender":"male"},
			{"Id":1,"Name":"Hilda Mayer","Age":21,"About":"Sit commodo consectetur minim amet ex. Elit aute mollit fugiat labore sint ipsum dolor cupidatat qui reprehenderit. Eu nisi in exercitation culpa sint aliqua nulla nulla proident eu. Nisi reprehenderit anim cupidatat dolor incididunt laboris mollit magna commodo ex. Cupidatat sit id aliqua amet nisi et voluptate voluptate commodo ex eiusmod et nulla velit.\n","Gender":"female"},
			{"Id":2,"Name":"Brooks Aguilar","Age":25,"About":"Velit ullamco est aliqua voluptate nisi do. Voluptate magna anim qui cillum aliqua sint veniam reprehenderit consectetur enim. Laborum dolore ut eiusmod ipsum ad anim est do tempor culpa ad do tempor. Nulla id aliqua dolore dolore adipisicing.\n","Gender":"male"},
			{"Id":3,"Name":"Everett Dillard","Age":27,"About":"Sint eu id sint irure officia amet cillum. Amet consectetur enim mollit culpa laborum ipsum adipisicing est laboris. Adipisicing fugiat esse dolore aliquip quis laborum aliquip dolore. Pariatur do elit eu nostrud occaecat.\n","Gender":"male"},
			{"Id":4,"Name":"Owen Lynn","Age":30,"About":"Elit anim elit eu et deserunt veniam laborum commodo irure nisi ut labore reprehenderit fugiat. Ipsum adipisicing labore ullamco occaecat ut. Ea deserunt ad dolor eiusmod aute non enim adipisicing sit ullamco est ullamco. Elit in proident pariatur elit ullamco quis. Exercitation amet nisi fugiat voluptate esse sit et consequat sit pariatur labore et.\n","Gender":"male"},
			{"Id":5,"Name":"Beulah Stark","Age":30,"About":"Enim cillum eu cillum velit labore. In sint esse nulla occaecat voluptate pariatur aliqua aliqua non officia nulla aliqua. Fugiat nostrud irure officia minim cupidatat laborum ad incididunt dolore. Fugiat nostrud eiusmod ex ea nulla commodo. Reprehenderit sint qui anim non ad id adipisicing qui officia Lorem.\n","Gender":"female"},
			{"Id":6,"Name":"Jennings Mays","Age":39,"About":"Veniam consectetur non non aliquip exercitation quis qui. Aliquip duis ut ad commodo consequat ipsum cupidatat id anim voluptate deserunt enim laboris. Sunt nostrud voluptate do est tempor esse anim pariatur. Ea do amet Lorem in mollit ipsum irure Lorem exercitation. Exercitation deserunt adipisicing nulla aute ex amet sint tempor incididunt magna. Quis et consectetur dolor nulla reprehenderit culpa laboris voluptate ut mollit. Qui ipsum nisi ullamco sit exercitation nisi magna fugiat anim consectetur officia.\n","Gender":"male"},
			{"Id":7,"Name":"Leann Travis","Age":34,"About":"Lorem magna dolore et velit ut officia. Cupidatat deserunt elit mollit amet nulla voluptate sit. Quis aute aliquip officia deserunt sint sint nisi. Laboris sit et ea dolore consequat laboris non. Consequat do enim excepteur qui mollit consectetur eiusmod laborum ut duis mollit dolor est. Excepteur amet duis enim laborum aliqua nulla ea minim.\n","Gender":"female"},
			{"Id":8,"Name":"Glenn Jordan","Age":29,"About":"Duis reprehenderit sit velit exercitation non aliqua magna quis ad excepteur anim. Eu cillum cupidatat sit magna cillum irure occaecat sunt officia officia deserunt irure. Cupidatat dolor cupidatat ipsum minim consequat Lorem adipisicing. Labore fugiat cupidatat nostrud voluptate ea eu pariatur non. Ipsum quis occaecat irure amet esse eu fugiat deserunt incididunt Lorem esse duis occaecat mollit.\n","Gender":"male"},
			{"Id":9,"Name":"Rose Carney","Age":36,"About":"Voluptate ipsum ad consequat elit ipsum tempor irure consectetur amet. Et veniam sunt in sunt ipsum non elit ullamco est est eu. Exercitation ipsum do deserunt do eu adipisicing id deserunt duis nulla ullamco eu. Ad duis voluptate amet quis commodo nostrud occaecat minim occaecat commodo. Irure sint incididunt est cupidatat laborum in duis enim nulla duis ut in ut. Cupidatat ex incididunt do ullamco do laboris eiusmod quis nostrud excepteur quis ea.\n","Gender":"female"},
			{"Id":10,"Name":"Henderson Maxwell","Age":30,"About":"Ex et excepteur anim in eiusmod. Cupidatat sunt aliquip exercitation velit minim aliqua ad ipsum cillum dolor do sit dolore cillum. Exercitation eu in ex qui voluptate fugiat amet.\n","Gender":"male"},
			{"Id":11,"Name":"Gilmore Guerra","Age":32,"About":"Labore consectetur do sit et mollit non incididunt. Amet aute voluptate enim et sit Lorem elit. Fugiat proident ullamco ullamco sint pariatur deserunt eu nulla consectetur culpa eiusmod. Veniam irure et deserunt consectetur incididunt ad ipsum sint. Consectetur voluptate adipisicing aute fugiat aliquip culpa qui nisi ut ex esse ex. Sint et anim aliqua pariatur.\n","Gender":"male"},
			{"Id":12,"Name":"Cruz Guerrero","Age":36,"About":"Sunt enim ad fugiat minim id esse proident laborum magna magna. Velit anim aliqua nulla laborum consequat veniam reprehenderit enim fugiat ipsum mollit nisi. Nisi do reprehenderit aute sint sit culpa id Lorem proident id tempor. Irure ut ipsum sit non quis aliqua in voluptate magna. Ipsum non aliquip quis incididunt incididunt aute sint. Minim dolor in mollit aute duis consectetur.\n","Gender":"male"},
			{"Id":13,"Name":"Whitley Davidson","Age":40,"About":"Consectetur dolore anim veniam aliqua deserunt officia eu. Et ullamco commodo ad officia duis ex incididunt proident consequat nostrud proident quis tempor. Sunt magna ad excepteur eu sint aliqua eiusmod deserunt proident. Do labore est dolore voluptate ullamco est dolore excepteur magna duis quis. Quis laborum deserunt ipsum velit occaecat est laborum enim aute. Officia dolore sit voluptate quis mollit veniam. Laborum nisi ullamco nisi sit nulla cillum et id nisi.\n","Gender":"male"},
			{"Id":14,"Name":"Nicholson Newman","Age":23,"About":"Tempor minim reprehenderit dolore et ad. Irure id fugiat incididunt do amet veniam ex consequat. Quis ad ipsum excepteur eiusmod mollit nulla amet velit quis duis ut irure.\n","Gender":"male"},
			{"Id":15,"Name":"Allison Valdez","Age":21,"About":"Labore excepteur voluptate velit occaecat est nisi minim. Laborum ea et irure nostrud enim sit incididunt reprehenderit id est nostrud eu. Ullamco sint nisi voluptate cillum nostrud aliquip et minim. Enim duis esse do aute qui officia ipsum ut occaecat deserunt. Pariatur pariatur nisi do ad dolore reprehenderit et et enim esse dolor qui. Excepteur ullamco adipisicing qui adipisicing tempor minim aliquip.\n","Gender":"male"},
			{"Id":16,"Name":"Annie Osborn","Age":35,"About":"Consequat fugiat veniam commodo nisi nostrud culpa pariatur. Aliquip velit adipisicing dolor et nostrud. Eu nostrud officia velit eiusmod ullamco duis eiusmod ad non do quis.\n","Gender":"female"},
			{"Id":17,"Name":"Dillard Mccoy","Age":36,"About":"Laborum voluptate sit ipsum tempor dolore. Adipisicing reprehenderit minim aliqua est. Consectetur enim deserunt incididunt elit non consectetur nisi esse ut dolore officia do ipsum.\n","Gender":"male"},
			{"Id":18,"Name":"Terrell Hall","Age":27,"About":"Ut nostrud est est elit incididunt consequat sunt ut aliqua sunt sunt. Quis consectetur amet occaecat nostrud duis. Fugiat in irure consequat laborum ipsum tempor non deserunt laboris id ullamco cupidatat sit. Officia cupidatat aliqua veniam et ipsum labore eu do aliquip elit cillum. Labore culpa exercitation sint sint.\n","Gender":"male"},
			{"Id":19,"Name":"Bell Bauer","Age":26,"About":"Nulla voluptate nostrud nostrud do ut tempor et quis non aliqua cillum in duis. Sit ipsum sit ut non proident exercitation. Quis consequat laboris deserunt adipisicing eiusmod non cillum magna.\n","Gender":"male"},
			{"Id":20,"Name":"Lowery York","Age":27,"About":"Dolor enim sit id dolore enim sint nostrud deserunt. Occaecat minim enim veniam proident mollit Lorem irure ex. Adipisicing pariatur adipisicing aliqua amet proident velit. Magna commodo culpa sit id.\n","Gender":"male"},
			{"Id":21,"Name":"Johns Whitney","Age":26,"About":"Elit sunt exercitation incididunt est ea quis do ad magna. Commodo laboris nisi aliqua eu incididunt eu irure. Labore ullamco quis deserunt non cupidatat sint aute in incididunt deserunt elit velit. Duis est mollit veniam aliquip. Nulla sunt veniam anim et sint dolore.\n","Gender":"male"},
			{"Id":22,"Name":"Beth Wynn","Age":31,"About":"Proident non nisi dolore id non. Aliquip ex anim cupidatat dolore amet veniam tempor non adipisicing. Aliqua adipisicing eu esse quis reprehenderit est irure cillum duis dolor ex. Laborum do aute commodo amet. Fugiat aute in excepteur ut aliqua sint fugiat do nostrud voluptate duis do deserunt. Elit esse ipsum duis ipsum.\n","Gender":"female"},
			{"Id":23,"Name":"Gates Spencer","Age":21,"About":"Dolore magna magna commodo irure. Proident culpa nisi veniam excepteur sunt qui et laborum tempor. Qui proident Lorem commodo dolore ipsum.\n","Gender":"male"},
			{"Id":24,"Name":"Gonzalez Anderson","Age":33,"About":"Quis consequat incididunt in ex deserunt minim aliqua ea duis. Culpa nisi excepteur sint est fugiat cupidatat nulla magna do id dolore laboris. Aute cillum eiusmod do amet dolore labore commodo do pariatur sit id. Do irure eiusmod reprehenderit non in duis sunt ex. Labore commodo labore pariatur ex minim qui sit elit.\n","Gender":"male"}
		],"NextPage":true}`,
	}
	testName := []string{
		"Last page, Order by Id desc",
		"Limit 1, Offset 1, Order by Age Asc",
		"Limit 26",
	}
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	client := new(SearchClient)
	client.URL = ts.URL

	for i := 0; i < len(testReq); i++ {
		result, err := client.FindUsers(testReq[i])

		if err != nil {
			t.Errorf("Error in test %s, got error %#v", testName[i], err)
			continue
		}
		// r2, _ := json.Marshal(result)
		// fmt.Println(string(r2))
		// fmt.Println(testResp[i])
		var data SearchResponse
		json.Unmarshal([]byte(testResp[i]), &data)
		if !reflect.DeepEqual(&data, result) {
			t.Errorf("Error in test %s, got data %#v, expected %#v", testName[i], result, data)
			break
		}
	}

	ts.Close()
}
