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
type Propagation int
const (
	Required    Propagation = iota //(默认)如果当前有事务，就用当前事务。如果当前没有事务，就新建一个事务 。have tx ? join : new tx()
	Support                        //支持当前事务，如果当前没有事务，就以非事务方式执行。  have tx ? join(): session.exec()
	Mandatory                      //支持当前事务，如果当前没有事务，则返回事务嵌套错误。  have tx ? join() : return error
	News                           //新建一个全新Session开启一个全新事务，如果当前存在事务，则把当前事务挂起。 have tx ? stop old。  -> new session().new tx()
	NotSupport                     //以非事务方式执行操作，如果当前存在事务，则新建一个Session以非事务方式执行操作，并把当前事务挂起。  have tx ? stop old。 -> new session().exec()
	Never                          //以非事务方式执行操作，如果当前存在事务，则返回事务嵌套错误。    have tx ? return error: session.exec()
	Nested                         //如果当前事务存在，则在嵌套事务内执行，如嵌套事务回滚，则只会在嵌套事务内回滚，不会影响当前事务。如果当前没有事务，则进行与Required类似的操作。
)
func toString(propagation Propagation) string {
	switch propagation {
	case Required:
		return "required"
		break
	case Support:
		return "support"
		break
	case Mandatory:
		return "mandatory"
		break
	case News:
		return "new"
		break
	case NotSupport:
		return "!support"
		break
	case Never:
		return "never"
		break
	case Nested:
		return "nested"
		break
	}
	return ""
}
func newPro(arg string) Propagation {
	switch arg {
	case "":
		return Required
		break
	case "required":
		return Required
		break
	case "support":
		return Support
		break
	case "mandatory":
		return Mandatory
		break
	case "new":
		return News
		break
	case "!support":
		return NotSupport
		break
	case "never":
		return Never
		break
	case "nested":
		return Nested
		break
	default:
		return Required
	}
	return Required
}
func printStr(str string)string{
	switch str {
	case "":
		return "当前有事务，就用当前事务。当前没有事务，就新建一个事务。"
		break
	case "required":
		return "当前有事务，就用当前事务。当前没有事务，就新建一个事务。"
		break
	case "support":
		return "支持当前事务，如果当前没有事务，就以非事务方式执行。"
		break
	case "mandatory":
		return "支持当前事务，如果当前没有事务，则返回事务嵌套错误。"
		break
	case "new":
		return "新建一个全新Session开启一个全新事务，如果当前存在事务，则把当前事务挂起。"
		break
	case "!support":
		return "以非事务方式执行操作，如果当前存在事务，则新建一个Session以非事务方式执行操作，并把当前事务挂起。"
		break
	case "never":
		return "以非事务方式执行操作，如果当前存在事务，则返回事务嵌套错误。"
		break
	case "nested":
		return "(定义嵌套事务)如果当前事务存在,则在嵌套事务内执行,如嵌套事务回滚,则只会在嵌套事务内回滚,不会影响当前事务.如果当前没有事务,则进行与Required类似的操作"
		break
	default:
		return "如果当前事务存在,则支持当前事务.否则,会启动一个新的事务"
	}
	return "当前有事务，就用当前事务。当前没有事务，就新建一个事务。"
}
type savePoint struct {
	i    int
	data []string
}
func newSavePoint()*savePoint{
	return &savePoint{
		data: []string{},
		i:    0,
	}
}
func (s *savePoint) Push(k string) {
	s.data = append(s.data, k)
	s.i++
}
func (s *savePoint) Pop() *string {
	if s.i == 0 {
		return nil
	}
	s.i--
	ret := s.data[s.i]
	s.data = s.data[0:s.i]
	return &ret
}
func (s *savePoint) Len() int {
	return s.i
}
type txStack struct {
	i            int
	data         []*sql.Tx
	propagations []*Propagation
}
func newTxStack() *txStack {
	return &txStack{
		data:         []*sql.Tx{},
		propagations: []*Propagation{},
		i:            0,
	}
}
func (s *txStack) Push(k *sql.Tx, p *Propagation) {
	s.data = append(s.data, k)
	s.propagations = append(s.propagations, p)
	s.i++
}
func (s *txStack) Pop() (*sql.Tx, *Propagation) {
	if s.i == 0 {
		return nil, nil
	}
	s.i--
	ret := s.data[s.i]
	s.data = s.data[0:s.i]
	p := s.propagations[s.i]
	s.propagations = s.propagations[0:s.i]
	return ret, p
}
func (s *txStack) First() (*sql.Tx, *Propagation) {
	if s.i == 0 {
		return nil, nil
	}
	ret := s.data[0]
	p := s.propagations[0]
	return ret, p
}
func (s *txStack) Last() (*sql.Tx, *Propagation) {
	if s.i == 0 {
		return nil, nil
	}
	ret := s.data[s.i-1]
	p := s.propagations[s.i-1]
	return ret, p
}
func (s *txStack) Len() int {
	return s.i
}
func (s *txStack) HaveTx() bool {
	return s.Len() > 0
}
type Session interface {
	query(sqlOrArgs string) ([]map[string][]byte, error)
	exec(sqlOrArgs string) (*result, error)
	queryPrepare(sqlPrepare string, args ...interface{}) ([]map[string][]byte, error)
	execPrepare(sqlPrepare string, args ...interface{}) (*result, error)
	id() string
	last() *Propagation
	stmtConvert() iConvert
	Begin(p *Propagation) error
	Commit() error
	Rollback() error
}
type session struct {
	db         *sql.DB
	stmt       *sql.Stmt
	txStack    *txStack
	savePoint  *savePoint
	child      *session
	log        *log.Logger
	SessionId  string
	driverType string
	dsn        string
	isClosed   bool
	printLog   bool
}
func newSession(driverType string, dsn string, db *sql.DB,log *log.Logger ,print bool)*session{
	return &session{
		SessionId:  newUUID().String(),
		db:         db,
		txStack:    newTxStack(),
		driverType: driverType,
		dsn:        dsn,
		printLog:   print,
		log:        log,
	}
}
func (it *session) id() string {
	return it.SessionId
}
func (it *session) Rollback() error {
	if it.child != nil {
		err := it.child.Rollback()
		it.child = nil
		if err != nil {
			return err
		}
	}
	t, p := it.txStack.Pop()
	if t != nil && p != nil {
		// 只有定义的是嵌套事务才一起回滚
		if *p == Nested {
			if it.savePoint == nil {
				it.savePoint = newSavePoint()
			}
			point := it.savePoint.Pop()
			if point != nil {
				if it.printLog {
					it.log.Println(" [" + it.id() + "] exec ====================" + "rollback to " + *point)
				}
				_, e := t.Exec("rollback to " + *point)
				if e != nil {
					return e
				}
			}
		}
		if it.txStack.Len() == 0 {
			if it.printLog {
				it.log.Println(" Rollback() Session : "+"[",it.id(),"]")
			}
			err := t.Rollback()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func (it *session) Commit() error {
	if it.child != nil {
		e := it.child.Commit()
		it.child = nil
		if e != nil {
			return e
		}
	}
	t, p := it.txStack.Pop()
	if t != nil && p != nil {
		if *p == Nested {
			if it.savePoint == nil {
				it.savePoint = newSavePoint()
			}
			pId := "p" + strconv.Itoa(it.txStack.Len()+1)
			it.savePoint.Push(pId)
			if it.printLog {
				it.log.Println("[" ,it.id(),"] exec "+"savepoint"+pId)
			}
			_, e := t.Exec("savepoint"+pId)
			if e != nil {
				return e
			}
		}
		if it.txStack.Len() == 0 {
			if it.printLog {
				it.log.Println(" Commit() tx Session : "+"[",it.id(),"]")
			}
			err := t.Commit()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func (it *session) Begin(p *Propagation) error {
	if p != nil {
		pro := toString(*p)
		note := printStr(pro)
		if it.printLog {
			it.log.Println(" [" + it.id() + "] Begin Session (Propagation : " + pro + ")"+" : "+note)
		}
		switch *p {
		case Required:
			if it.txStack.Len() > 0 {
				it.txStack.Push(it.txStack.Last())
				return nil
			} else {
				t, _ := it.db.Begin()
				it.txStack.Push(t, p)
				return nil
			}
			break
		case Support:
			if it.txStack.Len() > 0 {
				t, _ := it.db.Begin()
				it.txStack.Push(t, p)
				return nil
			} else {
				return nil
			}
			break
		case Mandatory:
			if it.txStack.Len() > 0 {
				t, _ := it.db.Begin()
				it.txStack.Push(t, p)
				return nil
			} else {
				return errors.New(" [Error] PropagationMandatory 当前没有事务,请定义一个事务!")
			}
			break
		case News:
			db, e := sql.Open(it.driverType, it.dsn)
			if e != nil {
				return e
			}
			it.child = newSession(it.driverType,it.dsn,db,it.log,it.printLog)
			break
		case NotSupport:
			db, e := sql.Open(it.driverType, it.dsn)
			if e != nil {
				return e
			}
			it.child = newSession(it.driverType,it.dsn,db,it.log,it.printLog)
			break
		case Never:
			if it.txStack.Len() > 0 {
				return errors.New(" [Error] 当前方法只能以非事务方式运行,不允许有任何事务!")
			}
			break
		case Nested:
			if it.savePoint == nil {
				it.savePoint = newSavePoint()
			}
			if it.txStack.Len() > 0 {
				it.txStack.Push(it.txStack.Last())
				return nil
			} else {
				np := Required
				return it.Begin(&np)
			}
			break
		default:
			panic("[Error] 请定义合适的事务类型!")
			break
		}
	}
	return nil
}
func (it *session) last() *Propagation {
	if it.txStack.Len() != 0 {
		_, pr := it.txStack.Last()
		return pr
	}
	return nil
}
func (it *session) query(args string) ([]map[string][]byte, error) {
	if it.child != nil {
		return it.child.query(args)
	}
	var (
		rows *sql.Rows
		err error
		t, _ = it.txStack.Last()
	)
	if t != nil {
		rows, err = t.Query(args)
	} else {
		rows, err = it.db.Query(args)
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
func (it *session) exec(args string) (*result, error) {
	if it.child != nil {
		return it.child.exec(args)
	}
	var (
		res sql.Result
		err error
		t, _ = it.txStack.Last()
	)
	if t != nil {
		res, err = t.Exec(args)
	} else {
		res, err = it.db.Exec(args)
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
func (it *session) queryPrepare(sqlPrepare string, args ...interface{}) ([]map[string][]byte, error) {
	if it.child != nil {
		return it.child.query(sqlPrepare)
	}
	var (
		rows *sql.Rows
		stmt *sql.Stmt
		err error
		t, _ = it.txStack.Last()
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
	if it.child != nil {
		return it.child.exec(sqlPrepare)
	}
	var (
		res sql.Result
		stmt *sql.Stmt
		err error
		t, _ = it.txStack.Last()
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
	if err := rows.Scan(list...); err != nil {
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



