package mbt

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)
var (
	epoch int64 = 1288834974657
	nodeBits uint8 = 10
	stepBits uint8 = 12
	mu        sync.Mutex
	nodeMax   int64 = -1 ^ (-1 << nodeBits)
	nodeMask        = nodeMax << stepBits
	stepMask  int64 = -1 ^ (-1 << stepBits)
	timeShift       = nodeBits + stepBits
	nodeShift       = stepBits
	decodeBase32Map [256]byte
	decodeBase58Map [256]byte
	errInvalidBase58 = errors.New("invalid base58")
	errInvalidBase32 = errors.New("invalid base32")
)
const (
	encodeBase32Map = "ybndrfg8ejkmcpqxot1uwisza345h769"
	encodeBase58Map = "123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
)
type (
	syntaxError struct{ original []byte }
	Id int64
)
func (j syntaxError) Error() string {
	return fmt.Sprintf("invalid snowflake ID %q", string(j.original))
}
func init() {
	for i := 0; i < len(encodeBase58Map); i++ {
		decodeBase58Map[i] = 0xFF
	}
	for i := 0; i < len(encodeBase58Map); i++ {
		decodeBase58Map[encodeBase58Map[i]] = byte(i)
	}
	for i := 0; i < len(encodeBase32Map); i++ {
		decodeBase32Map[i] = 0xFF
	}
	for i := 0; i < len(encodeBase32Map); i++ {
		decodeBase32Map[encodeBase32Map[i]] = byte(i)
	}
}
type node struct {
	mu    sync.Mutex
	epoch time.Time
	time  int64
	num  int64
	step  int64
	nodeMax   int64
	nodeMask  int64
	stepMask  int64
	timeShift uint8
	nodeShift uint8
}
func newNode(num int64) (*node, error) {
	mu.Lock()
	nodeMax = -1 ^ (-1 << nodeBits)
	nodeMask = nodeMax << stepBits
	stepMask = -1 ^ (-1 << stepBits)
	timeShift = nodeBits + stepBits
	nodeShift = stepBits
	mu.Unlock()
	n := node{}
	n.num = num
	n.nodeMax = -1 ^ (-1 << nodeBits)
	n.nodeMask = n.nodeMax << stepBits
	n.stepMask = -1 ^ (-1 << stepBits)
	n.timeShift = nodeBits + stepBits
	n.nodeShift = stepBits
	if n.num < 0 || n.num > n.nodeMax {
		return nil, errors.New("Node number must be between 0 and " + strconv.FormatInt(n.nodeMax, 10))
	}
	curTime := time.Now()
	n.epoch = curTime.Add(time.Unix(epoch/1000, (epoch%1000)*1000000).Sub(curTime))
	return &n, nil
}
func (n *node) Generate() Id {
	n.mu.Lock()
	now := time.Since(n.epoch).Nanoseconds() / 1000000
	if now == n.time {
		n.step = (n.step + 1) & n.stepMask
		if n.step == 0 {
			for now <= n.time {
				now = time.Since(n.epoch).Nanoseconds() / 1000000
			}
		}
	} else {
		n.step = 0
	}
	n.time = now
	r := Id((now)<<n.timeShift |
		(n.num << n.nodeShift) |
		(n.step),
	)
	n.mu.Unlock()
	return r
}
// Int64 returns an int64 of the snowflake ID
func (f Id) Int64() int64 {
	return int64(f)
}
// String returns a string of the snowflake ID
func (f Id) String() string {
	return strconv.FormatInt(int64(f), 10)
}
// Base2 returns a string base2 of the snowflake ID
func (f Id) Base2() string {
	return strconv.FormatInt(int64(f), 2)
}
// Base32 uses the z-base-32 character set but encodes and decodes similar
// to base58, allowing it to create an even smaller result string.
// NOTE: There are many different base32 implementations so becareful when
// doing any interoperation.
func (f Id) Base32() string {
	if f < 32 {
		return string(encodeBase32Map[f])
	}
	b := make([]byte, 0, 12)
	for f >= 32 {
		b = append(b, encodeBase32Map[f%32])
		f /= 32
	}
	b = append(b, encodeBase32Map[f])
	for x, y := 0, len(b)-1; x < y; x, y = x+1, y-1 {
		b[x], b[y] = b[y], b[x]
	}
	return string(b)
}
// Base36 returns a base36 string of the snowflake ID
func (f Id) Base36() string {
	return strconv.FormatInt(int64(f), 36)
}
// Base58 returns a base58 string of the snowflake ID
func (f Id) Base58() string {
	if f < 58 {
		return string(encodeBase58Map[f])
	}
	b := make([]byte, 0, 11)
	for f >= 58 {
		b = append(b, encodeBase58Map[f%58])
		f /= 58
	}
	b = append(b, encodeBase58Map[f])
	for x, y := 0, len(b)-1; x < y; x, y = x+1, y-1 {
		b[x], b[y] = b[y], b[x]
	}
	return string(b)
}
// Base64 returns a base64 string of the snowflake ID
func (f Id) Base64() string {
	return base64.StdEncoding.EncodeToString(f.Bytes())
}
// Bytes returns a byte slice of the snowflake ID
func (f Id) Bytes() []byte {
	return []byte(f.String())
}
// IntBytes returns an array of bytes of the snowflake ID, encoded as a
// big endian integer.
func (f Id) IntBytes() [8]byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(f))
	return b
}
// Time returns an int64 unix timestamp in milliseconds of the snowflake ID time
// DEPRECATED: the below function will be removed in a future release.
func (f Id) Time() int64 {
	return (int64(f) >> timeShift) + epoch
}
// Node returns an int64 of the snowflake ID node number
// DEPRECATED: the below function will be removed in a future release.
func (f Id) Node() int64 {
	return int64(f) & nodeMask >> nodeShift
}
// Step returns an int64 of the snowflake step (or sequence) number
// DEPRECATED: the below function will be removed in a future release.
func (f Id) Step() int64 {
	return int64(f) & stepMask
}
// MarshalJSON returns a json byte array string of the snowflake ID.
func (f Id) MarshalJSON() ([]byte, error) {
	buff := make([]byte, 0, 22)
	buff = append(buff, '"')
	buff = strconv.AppendInt(buff, int64(f), 10)
	buff = append(buff, '"')
	return buff, nil
}
// UnmarshalJSON converts a json byte array of a snowflake ID into an ID type.
func (f *Id) UnmarshalJSON(b []byte) error {
	if len(b) < 3 || b[0] != '"' || b[len(b)-1] != '"' {
		return syntaxError{b}
	}
	i, err := strconv.ParseInt(string(b[1:len(b)-1]), 10, 64)
	if err != nil {
		return err
	}
	*f = Id(i)
	return nil
}
