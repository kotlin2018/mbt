package mbt

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
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
func (it *session) len() int {
	return it.i
}
func (it *session) last() (*sql.Tx,string) {
	if it.i == 0 {
		return nil,"0"
	}
	ret := it.tx[it.i-1]
	p := it.propagation[it.i-1]
	return ret,p
}
func (it *session) pop() (*sql.Tx,string) {
	if it.i == 0 {
		return nil,"0"
	}
	it.i--
	ret := it.tx[it.i]
	it.tx = it.tx[0:it.i]
	p := it.propagation[it.i]
	it.propagation = it.propagation[0:it.i]
	return ret,p
}
func (it *session) push(k *sql.Tx,p string) {
	it.tx = append(it.tx, k)
	it.propagation = append(it.propagation,p)
	it.i++
}
func (it *session) sPush(k string) {
	it.data = append(it.data, k)
	it.si++
}
func (it *session) sPop() *string {
	if it.si == 0 {
		return nil
	}
	it.si--
	ret := it.data[it.si]
	it.data = it.data[0:it.si]
	return &ret
}
type Session interface {
	queryPrepare(sqlPrepare string, args ...interface{}) ([]map[string][]byte, error)
	execPrepare(sqlPrepare string, args ...interface{}) (*result, error)
	id() string
	begin(str string) error
	stmtConvert() iConvert
	Begin() error
	Commit() error
	Rollback() error
}
type session struct {
	db          *sql.DB
	tx          []*sql.Tx
	stmt        *sql.Stmt
	log         *log.Logger
	i           int
	si          int
	data        []string
	propagation []string
	SessionId   string
	driverType  string
	dsn         string
	isClosed    bool
	printLog    bool
}
func (it *session) id() string {
	return it.SessionId
}
func (it *session) Rollback() error {
	t, p := it.pop()
	if t != nil && p != "0" {
		if p == "nested" {
			point := it.sPop()
			if point != nil {
				_, e := t.Exec("rollback to " + *point)
				if e != nil {
					return e
				}
				if it.printLog {
					it.log.Println(" [" + it.id() + "] exec ====================" + "rollback to " + *point)
				}
			}
		}
		if it.len() == 0 {
			err := t.Rollback()
			if err != nil {
				return err
			}
			if it.printLog {
				it.log.Println(" Rollback() tx SessionId == "+"[",it.id(),"]")
			}
		}
	}
	return nil
}
func (it *session) Commit() error {
	t, p := it.pop()
	if t != nil && p != "0" {
		if p == "nested" {
			pId := "p" + strconv.Itoa(it.len()+1)
			it.sPush(pId)
			_, e := t.Exec("savepoint "+pId)
			if e != nil {
				return e
			}
			if it.printLog {
				it.log.Println("[" ,it.id(),"] exec "+"savepoint "+pId)
			}
		}
		if it.len() == 0 {
			err := t.Commit()
			if err != nil {
				return err
			}
			if it.printLog {
				it.log.Println(" Commit() tx SessionId == "+"[",it.id(),"]")
			}
		}
	}
	return nil
}
func (it *session) Begin() error {
	return it.begin("required")
}
func (it *session) begin(p string) error {
	if it.printLog {
		it.log.Println(" Begin tx "+"[",it.id(),"]")
	}
	switch p {
	case "","required":
		if it.len() > 0 {
			it.push(it.last())
			return nil
		} else {
			t, _ := it.db.Begin()
			it.push(t,p)
			return nil
		}
		break
	case "nested":
		it.si = 0
		it.data = make([]string,0)
		if it.len() > 0 {
			it.push(it.last())
			return nil
		}else {
			return it.begin("required")
		}
		break
	case "support":
		if it.len() > 0 {
			t, _ := it.db.Begin()
			it.push(t, p)
			return nil
		} else {
			return nil
		}
		break
	case "never":
		if it.len() > 0 {
			return errors.New(" [Error] 当前方法只能以非事务方式运行,不允许有任何事务!")
		}
		break
	case "mandatory":
		if it.len() > 0 {
			t, _ := it.db.Begin()
			it.push(t, p)
			return nil
		} else {
			return errors.New(" [Error] PropagationMandatory 当前没有事务,请定义一个事务!")
		}
		break
	}
	return nil
}
func (it *session) queryPrepare(sqlPrepare string, args ...interface{}) ([]map[string][]byte, error) {
	var (
		rows *sql.Rows
		stmt *sql.Stmt
		err error
		t,_= it.last()
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
	return nil, nil
}
func (it *session) execPrepare(sqlPrepare string, args ...interface{}) (*result, error) {
	var (
		res sql.Result
		stmt *sql.Stmt
		err error
		t,_ = it.last()
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
func (it *session) stmtConvert() iConvert {
	res,err := buildStmtConvert(it.driverType)
	if err != nil {
		it.log.SetPrefix("[Fatal] ")
		it.log.Fatalln(it.driverType+"error"+err.Error())
	}
	return res
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



