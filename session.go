package mbt

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"reflect"
	"strconv"
	"time"
)
type uuid [16]byte
var ra = rand.Reader
func (u uuid) String() string {
	var buf [36]byte
	encodeHex(buf[:], u)
	return string(buf[:])
}
func newUUID()uuid{
	var u uuid
	io.ReadFull(ra, u[:])
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}
func encodeHex(dst []byte, u uuid) {
	hex.Encode(dst, u[:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], u[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], u[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], u[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], u[10:])
}
func (it *Session) last() *sql.Tx {
	if it.i == 0 {
		return nil
	}
	return it.tx[it.i-1]
}
func (it *Session) pop() *sql.Tx {
	if it.i == 0 {
		return nil
	}
	it.i--
	ret := it.tx[it.i]
	it.tx = it.tx[0:it.i]
	return ret
}
func (it *Session) push(k *sql.Tx) {
	it.tx = append(it.tx, k)
	it.i++
}
type Session struct {
	db         *sql.DB
	tx         []*sql.Tx
	stmt       *sql.Stmt
	log        *log.Logger
	i          int
	sessionId  string
	driverName string
	dsn        string
	printLog   bool
}
func (it *Session) id() string {
	return it.sessionId
}
func (it *Session) Rollback() error {
	t := it.pop()
	if t != nil{
		err := t.Rollback()
		if err != nil {
			return err
		}
		if it.printLog {
			it.log.Println(" Rollback() tx SessionId == "+"[",it.id(),"]")
		}
	}
	return nil
}
func (it *Session) Commit() error {
	t := it.pop()
	if t != nil {
		err := t.Commit()
		if err != nil {
			return err
		}
		if it.printLog {
			it.log.Println(" Commit() tx SessionId == "+"[",it.id(),"]")
		}
	}
	return nil
}
func (it *Session) Begin() error {
	t, err := it.db.Begin()
	if err != nil {
		return err
	}
	it.push(t)
	if it.printLog {
		it.log.Println(" Begin() tx SessionId == "+"[",it.id(),"]")
	}
	return nil
}
func (it *Session) queryPrepare(sqlPrepare string, args ...interface{}) ([]map[string][]byte, error) {
	var (
		rows *sql.Rows
		stmt *sql.Stmt
		err error
		t = it.last()
	)
	if t != nil {
		stmt, err = t.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		rows, err = stmt.Query(args...)
		if err != nil {
			return nil, err
		}
	} else {
		stmt, err = it.db.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		rows, err = stmt.Query(args...)
		if err != nil {
			return nil, err
		}
	}
	if stmt != nil {
		defer stmt.Close()
	}
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		return nil, err
	} else {
		return rows2maps(rows)
	}
}
func (it *Session) execPrepare(sqlPrepare string, args ...interface{}) (*result, error) {
	var (
		res sql.Result
		stmt *sql.Stmt
		err error
		t = it.last()
	)
	if t != nil {
		stmt, err = t.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		res, err = stmt.Exec(args...)
		if err != nil {
			return nil, err
		}
	} else {
		stmt, err = it.db.Prepare(sqlPrepare)
		if err != nil {
			return nil, err
		}
		res, err = stmt.Exec(args...)
		if err != nil {
			return nil, err
		}
	}
	if stmt != nil {
		defer stmt.Close()
	}
	if err != nil {
		return nil, err
	} else {
		LastInsertId, _ := res.LastInsertId()
		RowsAffected, _ := res.RowsAffected()
		return &result{
			LastInsertId: LastInsertId,
			RowsAffected: RowsAffected,
		}, nil
	}
}
func rows2maps(rows *sql.Rows) (resultsSlice []map[string][]byte, err error) {
	fields, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		res, err := row2map(rows, fields)
		if err != nil {
			return nil, err
		}
		resultsSlice = append(resultsSlice, res)
	}
	return resultsSlice, nil
}
func row2map(rows *sql.Rows, fields []string) (resultsMap map[string][]byte, err error) {
	res := make(map[string][]byte)
	num := len(fields)
	list := make([]interface{},num)
	for i := 0; i < num; i++ {
		var obj interface{}
		list[i] = &obj
	}
	if err = rows.Scan(list...); err != nil {
		return nil, err
	}
	for j, v := range fields {
		rawValue := reflect.Indirect(reflect.ValueOf(list[j]))
		if rawValue.Interface() == nil {
			res[v] = []byte{}
			continue
		}
		if data, err := value2Bytes(&rawValue); err == nil {
			res[v] = data
		} else {
			return nil, err // REVIEW, should return err or just error logger?
		}
	}
	return res, nil
}
func value2Bytes(rawValue *reflect.Value) ([]byte, error) {
	var (
		str string
		err error
	)
	aa := reflect.TypeOf((*rawValue).Interface())
	vv := reflect.ValueOf((*rawValue).Interface())
	switch aa.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		str = strconv.FormatInt(vv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		str = strconv.FormatUint(vv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		str = strconv.FormatFloat(vv.Float(), 'f', -1, 64)
	case reflect.String:
		str = vv.String()
	case reflect.Array, reflect.Slice:
		switch aa.Elem().Kind() {
		case reflect.Uint8:
			data := rawValue.Interface().([]byte)
			str = string(data)
			if str == "\x00" {
				str = "0"
			}
		default:
			err = fmt.Errorf(" Unsupported struct type %v", vv.Type().Name())
		}
	case reflect.Struct:
		if aa.ConvertibleTo(timeType) {
			str = vv.Convert(timeType).Interface().(time.Time).Format(time.RFC3339Nano)
		} else {
			err = fmt.Errorf(" Unsupported struct type %v", vv.Type().Name())
		}
	case reflect.Bool:
		str = strconv.FormatBool(vv.Bool())
	case reflect.Complex128, reflect.Complex64:
		str = fmt.Sprintf("%v", vv.Complex())
	default:
		err = fmt.Errorf(" Unsupported struct type %v", vv.Type().Name())
	}
	return []byte(str),err
}



