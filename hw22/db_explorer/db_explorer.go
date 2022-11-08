package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

type FieldDescType struct {
	name            string
	fType           string
	isNull          bool
	isKey           bool
	isAutoincrement bool
}
type TableDescType struct {
	fields       []FieldDescType
	keyFieldName string
}
type DBDescType struct {
	tables map[string]TableDescType
	db     *sql.DB
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	dbDesc := initDBDesc(db)
	siteMux := http.NewServeMux()
	handler := func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			GetListHandler(w, req, dbDesc)
			return
		}
		rList := strings.SplitN(req.URL.Path, "/", 3)
		tableName := rList[1]
		recId := ""
		if len(rList) > 2 {
			recId = rList[2]
		}
		if len(rList) == 2 || recId == "" {
			if req.Method == "GET" {
				GetTableHandler(w, req, dbDesc, tableName)
			} else if req.Method == "PUT" {
				CreateRecordHandler(w, req, dbDesc, tableName)
			} else {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"error": "bad method"}`)
			}
			return
		}
		if req.Method == "GET" {
			GetRecordHandler(w, req, dbDesc, tableName, recId)
		} else if req.Method == "POST" {
			UpdateRecordHandler(w, req, dbDesc, tableName, recId)
		} else if req.Method == "DELETE" {
			DeleteRecordHandler(w, req, dbDesc, tableName, recId)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error": "bad method"}`)
		}
	}
	siteMux.HandleFunc("/", handler)

	return siteMux, nil
}

func initDBDesc(db *sql.DB) *DBDescType {
	ret := new(DBDescType)
	ret.tables = map[string]TableDescType{}
	ret.db = db

	for _, t := range getTableNames(db) {
		ret.tables[t] = getTableDesc(db, t)
	}

	return ret
}
func getTableNames(db *sql.DB) []string {
	ret := []string{}

	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var str string
		rows.Scan(&str)
		ret = append(ret, str)
	}

	return ret
}

func getTableDesc(db *sql.DB, tableName string) TableDescType {
	var ret TableDescType
	ret.fields = []FieldDescType{}

	rows, err := db.Query("SHOW FULL COLUMNS FROM `" + tableName + "`")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	ct, _ := rows.ColumnTypes()
	rLen := len(ct)
	// vals := make([]interface{}, rLen)
	vals := make([]interface{}, rLen)
	for rows.Next() {
		for i := 0; i < rLen; i++ {
			vals[i] = new(sql.NullString)
		}
		err := rows.Scan(vals...)
		if err != nil {
			panic(err)
		}

		fName := vals[0].(*sql.NullString).String
		fType := vals[1].(*sql.NullString).String
		switch fType[0] {
		case 'i':
			fType = "int"
		case 'f':
			fType = "float"
		default:
			fType = "string"
		}
		isNull := vals[3].(*sql.NullString).String == "YES"
		isKey := false
		if vals[4].(*sql.NullString).String == "PRI" {
			ret.keyFieldName = fName
			isKey = true
		}
		isAutoincrement := strings.Contains(vals[6].(*sql.NullString).String, "auto_increment")
		ret.fields = append(ret.fields, FieldDescType{
			name:            fName,
			fType:           fType,
			isNull:          isNull,
			isKey:           isKey,
			isAutoincrement: isAutoincrement,
		})
	}

	return ret
}

func GetListHandler(w http.ResponseWriter, req *http.Request, dbDesc *DBDescType) {
	if req.Method != "GET" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error": "bad method"}`)
		return
	}
	tables := []string{}
	for k := range dbDesc.tables {
		tables = append(tables, k)
	}
	sort.Strings(tables)
	response := map[string]interface{}{}
	response["tables"] = tables
	result := map[string]interface{}{}
	result["response"] = response
	responseBytes, _ := json.Marshal(result)
	w.Write(responseBytes)
}

func GetTableHandler(w http.ResponseWriter, req *http.Request, dbDesc *DBDescType, tableName string) {
	tableDesc, ok := dbDesc.tables[tableName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "unknown table"}`)
		return
	}
	req.ParseForm()
	mySql := "SELECT * FROM `" + tableName + "`"
	if req.Form.Has("limit") {
		limit, err := strconv.Atoi(req.FormValue("limit"))
		if err == nil && limit > 0 {
			mySql = mySql + " LIMIT " + strconv.Itoa(limit)
		}
		if req.Form.Has("offset") {
			offset, err := strconv.Atoi(req.FormValue("offset"))
			if err == nil && offset > 0 {
				mySql = mySql + " OFFSET " + strconv.Itoa(offset)
			}
		}
	}
	rows, err := dbDesc.db.Query(mySql)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}
	defer rows.Close()
	data := []interface{}{}
	rLen := len(tableDesc.fields)
	vals := make([]interface{}, rLen)
	for rows.Next() {
		for i := 0; i < rLen; i++ {
			switch tableDesc.fields[i].fType {
			case "int":
				vals[i] = new(sql.NullInt64)
			case "float":
				vals[i] = new(sql.NullFloat64)
			default:
				vals[i] = new(sql.NullString)
			}
		}
		err := rows.Scan(vals...)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"error": "`+err.Error()+`"}`)
			return
		}
		dataRow := map[string]interface{}{}
		var fVal interface{}
		for i := 0; i < rLen; i++ {
			switch tableDesc.fields[i].fType {
			case "int":
				if vals[i].(*sql.NullInt64).Valid {
					fVal = vals[i].(*sql.NullInt64).Int64
				} else {
					fVal = nil
				}
			case "float":
				if vals[i].(*sql.NullFloat64).Valid {
					fVal = vals[i].(*sql.NullFloat64).Float64
				} else {
					fVal = nil
				}
			default:
				if vals[i].(*sql.NullString).Valid {
					fVal = vals[i].(*sql.NullString).String
				} else {
					fVal = nil
				}
			}
			dataRow[tableDesc.fields[i].name] = fVal
		}
		data = append(data, dataRow)
	}

	response := map[string]interface{}{}
	response["records"] = data
	result := map[string]interface{}{}
	result["response"] = response
	responseBytes, _ := json.Marshal(result)
	w.Write(responseBytes)
}

func GetRecordHandler(w http.ResponseWriter, req *http.Request, dbDesc *DBDescType, tableName, recId string) {
	tableDesc, ok := dbDesc.tables[tableName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "unknown table"}`)
		return
	}
	mySql := "SELECT * FROM `" + tableName + "` WHERE `" + tableDesc.keyFieldName + "` = ?"
	rows, err := dbDesc.db.Query(mySql, recId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}
	defer rows.Close()
	data := []interface{}{}
	rLen := len(tableDesc.fields)
	vals := make([]interface{}, rLen)
	for rows.Next() {
		for i := 0; i < rLen; i++ {
			switch tableDesc.fields[i].fType {
			case "int":
				vals[i] = new(sql.NullInt64)
			case "float":
				vals[i] = new(sql.NullFloat64)
			default:
				vals[i] = new(sql.NullString)
			}
		}
		err := rows.Scan(vals...)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"error": "`+err.Error()+`"}`)
			return
		}
		dataRow := map[string]interface{}{}
		var fVal interface{}
		for i := 0; i < rLen; i++ {
			switch tableDesc.fields[i].fType {
			case "int":
				if vals[i].(*sql.NullInt64).Valid {
					fVal = vals[i].(*sql.NullInt64).Int64
				} else {
					fVal = nil
				}
			case "float":
				if vals[i].(*sql.NullFloat64).Valid {
					fVal = vals[i].(*sql.NullFloat64).Float64
				} else {
					fVal = nil
				}
			default:
				if vals[i].(*sql.NullString).Valid {
					fVal = vals[i].(*sql.NullString).String
				} else {
					fVal = nil
				}
			}
			dataRow[tableDesc.fields[i].name] = fVal
		}
		data = append(data, dataRow)
	}
	if len(data) == 0 {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "record not found"}`)
		return
	}

	response := map[string]interface{}{}
	response["record"] = data[0]
	result := map[string]interface{}{}
	result["response"] = response
	responseBytes, _ := json.Marshal(result)
	w.Write(responseBytes)
}

func CreateRecordHandler(w http.ResponseWriter, req *http.Request, dbDesc *DBDescType, tableName string) {
	tableDesc, ok := dbDesc.tables[tableName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "unknown table"}`)
		return
	}
	reqData := map[string]interface{}{}
	jsonDecoder := json.NewDecoder(req.Body)
	jsonDecoder.UseNumber()
	err := jsonDecoder.Decode(&reqData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}

	rLen := len(tableDesc.fields)
	data := []interface{}{}
	fieldsSQL := ""
	questionMarkSQL := ""
	isFirst := true
	for i := 0; i < rLen; i++ {
		if tableDesc.fields[i].isAutoincrement {
			continue
		}
		if !isFirst {
			fieldsSQL += ","
			questionMarkSQL += ","
		} else {
			isFirst = false
		}
		fieldsSQL += "`" + tableDesc.fields[i].name + "`"
		questionMarkSQL += "?"
		reqField, ok := reqData[tableDesc.fields[i].name]
		if (!ok || reqField == nil) && !tableDesc.fields[i].isNull {
			switch tableDesc.fields[i].fType {
			case "int":
				reqField = 0
			case "float":
				reqField = 0.0
			default:
				reqField = ""
			}
		}
		data = append(data, reqField)
	}
	mySQL := "INSERT INTO `" + tableName + "` (" + fieldsSQL + ") VALUES (" + questionMarkSQL + ")"
	res, err := dbDesc.db.Exec(mySQL, data...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}

	lastId, err := res.LastInsertId()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}

	response := map[string]interface{}{}
	response[tableDesc.keyFieldName] = int(lastId)
	result := map[string]interface{}{}
	result["response"] = response
	responseBytes, _ := json.Marshal(result)
	w.Write(responseBytes)
}

func UpdateRecordHandler(w http.ResponseWriter, req *http.Request, dbDesc *DBDescType, tableName, recId string) {
	tableDesc, ok := dbDesc.tables[tableName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "unknown table"}`)
		return
	}
	reqData := map[string]interface{}{}
	jsonDecoder := json.NewDecoder(req.Body)
	jsonDecoder.UseNumber()
	err := jsonDecoder.Decode(&reqData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}

	rLen := len(tableDesc.fields)
	data := []interface{}{}
	fieldsSQL := ""
	isFirst := true
	for i := 0; i < rLen; i++ {
		reqField, ok := reqData[tableDesc.fields[i].name]
		if !ok {
			continue
		}
		if tableDesc.fields[i].name == tableDesc.keyFieldName {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error": "field %s have invalid type"}`, tableDesc.keyFieldName)
			return
		}
		if reqField == nil {
			if !tableDesc.fields[i].isNull {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"error": "field %s have invalid type"}`, tableDesc.fields[i].name)
				return
			}
		} else {
			if tableDesc.fields[i].fType == "int" {
				_, ok := reqField.(int)
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, `{"error": "field %s have invalid type"}`, tableDesc.fields[i].name)
					return
				}
			} else if tableDesc.fields[i].fType == "float" {
				_, ok := reqField.(float64)
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, `{"error": "field %s have invalid type"}`, tableDesc.fields[i].name)
					return
				}
			} else {
				_, ok := reqField.(string)
				if !ok {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, `{"error": "field %s have invalid type"}`, tableDesc.fields[i].name)
					return
				}
			}
		}

		if !isFirst {
			fieldsSQL += ","
		} else {
			isFirst = false
		}
		fieldsSQL += "`" + tableDesc.fields[i].name + "`=? "
		data = append(data, reqField)
	}
	mySQL := "UPDATE `" + tableName + "` SET " + fieldsSQL + " WHERE `" + tableDesc.keyFieldName + "` = ?"
	data = append(data, recId)
	res, err := dbDesc.db.Exec(mySQL, data...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}

	rAffected, err := res.RowsAffected()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}
	if rAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "unknown table"}`)
		return
	}

	response := map[string]interface{}{}
	response["updated"] = int(rAffected)
	result := map[string]interface{}{}
	result["response"] = response
	responseBytes, _ := json.Marshal(result)
	w.Write(responseBytes)
}

func DeleteRecordHandler(w http.ResponseWriter, req *http.Request, dbDesc *DBDescType, tableName, recId string) {
	tableDesc, ok := dbDesc.tables[tableName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "unknown table"}`)
		return
	}
	mySQL := "DELETE FROM `" + tableName + "`  WHERE `" + tableDesc.keyFieldName + "` = ?"
	res, err := dbDesc.db.Exec(mySQL, recId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}

	rAffected, err := res.RowsAffected()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error": "`+err.Error()+`"}`)
		return
	}

	response := map[string]interface{}{}
	response["deleted"] = int(rAffected)
	result := map[string]interface{}{}
	result["response"] = response
	responseBytes, _ := json.Marshal(result)
	w.Write(responseBytes)
}
