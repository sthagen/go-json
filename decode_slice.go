package json

import (
	"reflect"
	"sync"
	"unsafe"
)

type sliceDecoder struct {
	elemType                *rtype
	isElemPointerType       bool
	isElemUnmarshalJSONType bool
	valueDecoder            decoder
	size                    uintptr
	arrayPool               sync.Pool
	structName              string
	fieldName               string
}

// If use reflect.SliceHeader, data type is uintptr.
// In this case, Go compiler cannot trace reference created by newArray().
// So, define using unsafe.Pointer as data type
type sliceHeader struct {
	data unsafe.Pointer
	len  int
	cap  int
}

const (
	defaultSliceCapacity = 2
)

func newSliceDecoder(dec decoder, elemType *rtype, size uintptr, structName, fieldName string) *sliceDecoder {
	return &sliceDecoder{
		valueDecoder:            dec,
		elemType:                elemType,
		isElemPointerType:       elemType.Kind() == reflect.Ptr || elemType.Kind() == reflect.Map,
		isElemUnmarshalJSONType: rtype_ptrTo(elemType).Implements(unmarshalJSONType),
		size:                    size,
		arrayPool: sync.Pool{
			New: func() interface{} {
				return &sliceHeader{
					data: newArray(elemType, defaultSliceCapacity),
					len:  0,
					cap:  defaultSliceCapacity,
				}
			},
		},
		structName: structName,
		fieldName:  fieldName,
	}
}

func (d *sliceDecoder) newSlice() *sliceHeader {
	slice := d.arrayPool.Get().(*sliceHeader)
	slice.len = 0
	return slice
}

func (d *sliceDecoder) releaseSlice(p *sliceHeader) {
	d.arrayPool.Put(p)
}

//go:linkname copySlice reflect.typedslicecopy
func copySlice(elemType *rtype, dst, src sliceHeader) int

//go:linkname newArray reflect.unsafe_NewArray
func newArray(*rtype, int) unsafe.Pointer

//go:linkname typedmemmove reflect.typedmemmove
func typedmemmove(t *rtype, dst, src unsafe.Pointer)

func (d *sliceDecoder) errNumber(offset int64) *UnmarshalTypeError {
	return &UnmarshalTypeError{
		Value:  "number",
		Type:   reflect.SliceOf(rtype2type(d.elemType)),
		Struct: d.structName,
		Field:  d.fieldName,
		Offset: offset,
	}
}

func (d *sliceDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	depth++
	if depth > maxDecodeNestingDepth {
		return errExceededMaxDepth(s.char(), s.cursor)
	}

	for {
		switch s.char() {
		case ' ', '\n', '\t', '\r':
			s.cursor++
			continue
		case 'n':
			if err := nullBytes(s); err != nil {
				return err
			}
			*(*unsafe.Pointer)(p) = nil
			return nil
		case '[':
			s.cursor++
			s.skipWhiteSpace()
			if s.char() == ']' {
				dst := (*sliceHeader)(p)
				if dst.data == nil {
					dst.data = newArray(d.elemType, 0)
				} else {
					dst.len = 0
				}
				s.cursor++
				return nil
			}
			idx := 0
			slice := d.newSlice()
			capacity := slice.cap
			data := slice.data
			for {
				if capacity <= idx {
					src := sliceHeader{data: data, len: idx, cap: capacity}
					capacity *= 2
					data = newArray(d.elemType, capacity)
					dst := sliceHeader{data: data, len: idx, cap: capacity}
					copySlice(d.elemType, dst, src)
				}
				ep := unsafe.Pointer(uintptr(data) + uintptr(idx)*d.size)
				if d.isElemUnmarshalJSONType {
					// assign new element to the slice
					typedmemmove(d.elemType, ep, unsafe_New(d.elemType))
				} else if d.isElemPointerType {
					**(**unsafe.Pointer)(unsafe.Pointer(&ep)) = nil // initialize elem pointer
				}
				if err := d.valueDecoder.decodeStream(s, depth, ep); err != nil {
					return err
				}
				s.skipWhiteSpace()
			RETRY:
				switch s.char() {
				case ']':
					slice.cap = capacity
					slice.len = idx + 1
					slice.data = data
					dst := (*sliceHeader)(p)
					dst.len = idx + 1
					if dst.len > dst.cap {
						dst.data = newArray(d.elemType, dst.len)
						dst.cap = dst.len
					}
					copySlice(d.elemType, *dst, *slice)
					d.releaseSlice(slice)
					s.cursor++
					return nil
				case ',':
					idx++
				case nul:
					if s.read() {
						goto RETRY
					}
					slice.cap = capacity
					slice.data = data
					d.releaseSlice(slice)
					goto ERROR
				default:
					slice.cap = capacity
					slice.data = data
					d.releaseSlice(slice)
					goto ERROR
				}
				s.cursor++
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return d.errNumber(s.totalOffset())
		case nul:
			if s.read() {
				continue
			}
			goto ERROR
		default:
			goto ERROR
		}
	}
ERROR:
	return errUnexpectedEndOfJSON("slice", s.totalOffset())
}

func (d *sliceDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	depth++
	if depth > maxDecodeNestingDepth {
		return 0, errExceededMaxDepth(buf[cursor], cursor)
	}

	buflen := int64(len(buf))
	for ; cursor < buflen; cursor++ {
		switch buf[cursor] {
		case ' ', '\n', '\t', '\r':
			continue
		case 'n':
			buflen := int64(len(buf))
			if cursor+3 >= buflen {
				return 0, errUnexpectedEndOfJSON("null", cursor)
			}
			if buf[cursor+1] != 'u' {
				return 0, errInvalidCharacter(buf[cursor+1], "null", cursor)
			}
			if buf[cursor+2] != 'l' {
				return 0, errInvalidCharacter(buf[cursor+2], "null", cursor)
			}
			if buf[cursor+3] != 'l' {
				return 0, errInvalidCharacter(buf[cursor+3], "null", cursor)
			}
			cursor += 4
			*(*unsafe.Pointer)(p) = nil
			return cursor, nil
		case '[':
			cursor++
			cursor = skipWhiteSpace(buf, cursor)
			if buf[cursor] == ']' {
				dst := (*sliceHeader)(p)
				if dst.data == nil {
					dst.data = newArray(d.elemType, 0)
				} else {
					dst.len = 0
				}
				cursor++
				return cursor, nil
			}
			idx := 0
			slice := d.newSlice()
			capacity := slice.cap
			data := slice.data
			for {
				if capacity <= idx {
					src := sliceHeader{data: data, len: idx, cap: capacity}
					capacity *= 2
					data = newArray(d.elemType, capacity)
					dst := sliceHeader{data: data, len: idx, cap: capacity}
					copySlice(d.elemType, dst, src)
				}
				ep := unsafe.Pointer(uintptr(data) + uintptr(idx)*d.size)
				if d.isElemUnmarshalJSONType {
					// assign new element to the slice
					typedmemmove(d.elemType, ep, unsafe_New(d.elemType))
				} else if d.isElemPointerType {
					**(**unsafe.Pointer)(unsafe.Pointer(&ep)) = nil // initialize elem pointer
				}
				c, err := d.valueDecoder.decode(buf, cursor, depth, ep)
				if err != nil {
					return 0, err
				}
				cursor = c
				cursor = skipWhiteSpace(buf, cursor)
				switch buf[cursor] {
				case ']':
					slice.cap = capacity
					slice.len = idx + 1
					slice.data = data
					dst := (*sliceHeader)(p)
					dst.len = idx + 1
					if dst.len > dst.cap {
						dst.data = newArray(d.elemType, dst.len)
						dst.cap = dst.len
					}
					copySlice(d.elemType, *dst, *slice)
					d.releaseSlice(slice)
					cursor++
					return cursor, nil
				case ',':
					idx++
				default:
					slice.cap = capacity
					slice.data = data
					d.releaseSlice(slice)
					return 0, errInvalidCharacter(buf[cursor], "slice", cursor)
				}
				cursor++
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return 0, d.errNumber(cursor)
		default:
			goto ERROR
		}
	}
ERROR:
	return 0, errUnexpectedEndOfJSON("slice", cursor)
}
