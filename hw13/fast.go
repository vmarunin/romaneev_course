package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

//easyjson:json
type UserType struct {
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Browsers []string `json:"browsers"`
}

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	seenBrowsers := map[string]bool{}

	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	fmt.Fprintln(out, "found users:")
	fscanner := bufio.NewScanner(file)
	for i := 0; fscanner.Scan(); i++ {
		user := UserType{}
		// fmt.Printf("%v %v\n", err, line)
		err := user.UnmarshalJSON(fscanner.Bytes())
		// err := json.Unmarshal(fscanner.Bytes(), &user)
		if err != nil {
			panic(err)
		}

		isAndroid := false
		isMSIE := false

		// browsers, ok := user["browsers"].([]interface{})
		// if !ok {
		if len(user.Browsers) == 0 {
			// log.Println("cant cast browsers")
			continue
		}

		// for _, browserRaw := range browsers {
		// browser, ok := browserRaw.(string)
		// if !ok {
		// log.Println("cant cast browser to string")
		// continue
		// }
		for _, browser := range user.Browsers {

			if strings.Contains(browser, "Android") {
				isAndroid = true
				seenBrowsers[browser] = true
			}
			if strings.Contains(browser, "MSIE") {
				isMSIE = true
				seenBrowsers[browser] = true
			}
		}

		if !(isAndroid && isMSIE) {
			continue
		}

		// log.Println("Android and MSIE user:", user["name"], user["email"])
		// email := strings.ReplaceAll(user["email"].(string), "@", " [at] ")
		//atPos := strings.IndexByte(user.Email, '@')

		email := strings.ReplaceAll(user.Email, "@", " [at] ")
		// fmt.Fprintf(out, "[%d] %s <%s>\n", i, user["name"], email)
		//fmt.Fprintf(out, "[%d] %s <%s [at] %s>\n", i, user.Name, user.Email[:atPos], user.Email[atPos+1:])
		fmt.Fprintf(out, "[%d] %s <%s>\n", i, user.Name, email)
	}

	fmt.Fprintln(out, "\nTotal unique browsers", len(seenBrowsers))
}
