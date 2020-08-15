package json

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

func (e *Encoder) compileHead(typ *rtype, withIndent bool) (*opcode, error) {
	if typ.Implements(marshalJSONType) {
		return newOpCode(opMarshalJSON, typ, e.indent, newEndOp(e.indent)), nil
	} else if typ.Implements(marshalTextType) {
		return newOpCode(opMarshalText, typ, e.indent, newEndOp(e.indent)), nil
	}
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	root := true
	if typ.Kind() == reflect.Map {
		return e.compileMap(typ, false, root, withIndent)
	}
	return e.compile(typ, root, withIndent)
}

func (e *Encoder) compile(typ *rtype, root, withIndent bool) (*opcode, error) {
	if typ.Implements(marshalJSONType) {
		return newOpCode(opMarshalJSON, typ, e.indent, newEndOp(e.indent)), nil
	} else if typ.Implements(marshalTextType) {
		return newOpCode(opMarshalText, typ, e.indent, newEndOp(e.indent)), nil
	}
	switch typ.Kind() {
	case reflect.Ptr:
		return e.compilePtr(typ, root, withIndent)
	case reflect.Slice:
		return e.compileSlice(typ, root, withIndent)
	case reflect.Array:
		return e.compileArray(typ, root, withIndent)
	case reflect.Map:
		return e.compileMap(typ, true, root, withIndent)
	case reflect.Struct:
		return e.compileStruct(typ, root, withIndent)
	case reflect.Interface:
		return e.compileInterface(typ, root)
	case reflect.Int:
		return e.compileInt(typ)
	case reflect.Int8:
		return e.compileInt8(typ)
	case reflect.Int16:
		return e.compileInt16(typ)
	case reflect.Int32:
		return e.compileInt32(typ)
	case reflect.Int64:
		return e.compileInt64(typ)
	case reflect.Uint:
		return e.compileUint(typ)
	case reflect.Uint8:
		return e.compileUint8(typ)
	case reflect.Uint16:
		return e.compileUint16(typ)
	case reflect.Uint32:
		return e.compileUint32(typ)
	case reflect.Uint64:
		return e.compileUint64(typ)
	case reflect.Uintptr:
		return e.compileUint(typ)
	case reflect.Float32:
		return e.compileFloat32(typ)
	case reflect.Float64:
		return e.compileFloat64(typ)
	case reflect.String:
		return e.compileString(typ)
	case reflect.Bool:
		return e.compileBool(typ)
	}
	return nil, &UnsupportedTypeError{Type: rtype2type(typ)}
}

func (e *Encoder) optimizeStructFieldPtrHead(typ *rtype, code *opcode) *opcode {
	ptrHeadOp := code.op.headToPtrHead()
	if code.op != ptrHeadOp {
		code.op = ptrHeadOp
		return code
	}
	return newOpCode(opPtr, typ, e.indent, code)
}

func (e *Encoder) compilePtr(typ *rtype, root, withIndent bool) (*opcode, error) {
	code, err := e.compile(typ.Elem(), root, withIndent)
	if err != nil {
		return nil, err
	}
	return e.optimizeStructFieldPtrHead(typ, code), nil
}

func (e *Encoder) compileInt(typ *rtype) (*opcode, error) {
	return newOpCode(opInt, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileInt8(typ *rtype) (*opcode, error) {
	return newOpCode(opInt8, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileInt16(typ *rtype) (*opcode, error) {
	return newOpCode(opInt16, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileInt32(typ *rtype) (*opcode, error) {
	return newOpCode(opInt32, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileInt64(typ *rtype) (*opcode, error) {
	return newOpCode(opInt64, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileUint(typ *rtype) (*opcode, error) {
	return newOpCode(opUint, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileUint8(typ *rtype) (*opcode, error) {
	return newOpCode(opUint8, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileUint16(typ *rtype) (*opcode, error) {
	return newOpCode(opUint16, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileUint32(typ *rtype) (*opcode, error) {
	return newOpCode(opUint32, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileUint64(typ *rtype) (*opcode, error) {
	return newOpCode(opUint64, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileFloat32(typ *rtype) (*opcode, error) {
	return newOpCode(opFloat32, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileFloat64(typ *rtype) (*opcode, error) {
	return newOpCode(opFloat64, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileString(typ *rtype) (*opcode, error) {
	return newOpCode(opString, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileBool(typ *rtype) (*opcode, error) {
	return newOpCode(opBool, typ, e.indent, newEndOp(e.indent)), nil
}

func (e *Encoder) compileInterface(typ *rtype, root bool) (*opcode, error) {
	return (*opcode)(unsafe.Pointer(&interfaceCode{
		opcodeHeader: &opcodeHeader{
			op:     opInterface,
			typ:    typ,
			indent: e.indent,
			next:   newEndOp(e.indent),
		},
		root: root,
	})), nil
}

func (e *Encoder) compileSlice(typ *rtype, root, withIndent bool) (*opcode, error) {
	elem := typ.Elem()
	size := elem.Size()

	e.indent++
	code, err := e.compile(elem, false, withIndent)
	e.indent--

	if err != nil {
		return nil, err
	}

	// header => opcode => elem => end
	//             ^        |
	//             |________|

	header := newSliceHeaderCode(e.indent)
	elemCode := &sliceElemCode{
		opcodeHeader: &opcodeHeader{
			op:     opSliceElem,
			indent: e.indent,
		},
		size: size,
	}
	end := newOpCode(opSliceEnd, nil, e.indent, newEndOp(e.indent))
	if withIndent {
		if root {
			header.op = opRootSliceHeadIndent
			elemCode.op = opRootSliceElemIndent
		} else {
			header.op = opSliceHeadIndent
			elemCode.op = opSliceElemIndent
		}
		end.op = opSliceEndIndent
	}

	header.elem = elemCode
	header.end = end
	header.next = code
	code.beforeLastCode().next = (*opcode)(unsafe.Pointer(elemCode))
	elemCode.next = code
	elemCode.end = end
	return (*opcode)(unsafe.Pointer(header)), nil
}

func (e *Encoder) compileArray(typ *rtype, root, withIndent bool) (*opcode, error) {
	elem := typ.Elem()
	alen := typ.Len()
	size := elem.Size()

	e.indent++
	code, err := e.compile(elem, false, withIndent)
	e.indent--

	if err != nil {
		return nil, err
	}
	// header => opcode => elem => end
	//             ^        |
	//             |________|

	header := newArrayHeaderCode(e.indent, alen)
	elemCode := &arrayElemCode{
		opcodeHeader: &opcodeHeader{
			op: opArrayElem,
		},
		len:  uintptr(alen),
		size: size,
	}
	end := newOpCode(opArrayEnd, nil, e.indent, newEndOp(e.indent))

	if withIndent {
		header.op = opArrayHeadIndent
		elemCode.op = opArrayElemIndent
		end.op = opArrayEndIndent
	}

	header.elem = elemCode
	header.end = end
	header.next = code
	code.beforeLastCode().next = (*opcode)(unsafe.Pointer(elemCode))
	elemCode.next = code
	elemCode.end = end
	return (*opcode)(unsafe.Pointer(header)), nil
}

//go:linkname mapiterinit reflect.mapiterinit
//go:noescape
func mapiterinit(mapType *rtype, m unsafe.Pointer) unsafe.Pointer

//go:linkname mapiterkey reflect.mapiterkey
//go:noescape
func mapiterkey(it unsafe.Pointer) unsafe.Pointer

//go:linkname mapiternext reflect.mapiternext
//go:noescape
func mapiternext(it unsafe.Pointer)

//go:linkname maplen reflect.maplen
//go:noescape
func maplen(m unsafe.Pointer) int

func (e *Encoder) compileMap(typ *rtype, withLoad, root, withIndent bool) (*opcode, error) {
	// header => code => value => code => key => code => value => code => end
	//                                     ^                       |
	//                                     |_______________________|
	e.indent++
	keyType := typ.Key()
	keyCode, err := e.compile(keyType, false, withIndent)
	if err != nil {
		return nil, err
	}
	valueType := typ.Elem()
	valueCode, err := e.compile(valueType, false, withIndent)
	if err != nil {
		return nil, err
	}

	key := newMapKeyCode(e.indent)
	value := newMapValueCode(e.indent)

	e.indent--

	header := newMapHeaderCode(typ, withLoad, e.indent)
	header.key = key
	header.value = value
	end := newOpCode(opMapEnd, nil, e.indent, newEndOp(e.indent))

	if withIndent {
		if header.op == opMapHead {
			if root {
				header.op = opRootMapHeadIndent
			} else {
				header.op = opMapHeadIndent
			}
		} else {
			header.op = opMapHeadLoadIndent
		}
		if root {
			key.op = opRootMapKeyIndent
		} else {
			key.op = opMapKeyIndent
		}
		value.op = opMapValueIndent
		end.op = opMapEndIndent
	}

	header.next = keyCode
	keyCode.beforeLastCode().next = (*opcode)(unsafe.Pointer(value))
	value.next = valueCode
	valueCode.beforeLastCode().next = (*opcode)(unsafe.Pointer(key))
	key.next = keyCode

	header.end = end
	key.end = end

	return (*opcode)(unsafe.Pointer(header)), nil
}

func (e *Encoder) getTag(field reflect.StructField) string {
	return field.Tag.Get("json")
}

func (e *Encoder) isIgnoredStructField(field reflect.StructField) bool {
	if field.PkgPath != "" && !field.Anonymous {
		// private field
		return true
	}
	tag := e.getTag(field)
	if tag == "-" {
		return true
	}
	return false
}

func (e *Encoder) typeToHeaderType(op opType) opType {
	switch op {
	case opInt:
		return opStructFieldHeadInt
	case opInt8:
		return opStructFieldHeadInt8
	case opInt16:
		return opStructFieldHeadInt16
	case opInt32:
		return opStructFieldHeadInt32
	case opInt64:
		return opStructFieldHeadInt64
	case opUint:
		return opStructFieldHeadUint
	case opUint8:
		return opStructFieldHeadUint8
	case opUint16:
		return opStructFieldHeadUint16
	case opUint32:
		return opStructFieldHeadUint32
	case opUint64:
		return opStructFieldHeadUint64
	case opFloat32:
		return opStructFieldHeadFloat32
	case opFloat64:
		return opStructFieldHeadFloat64
	case opString:
		return opStructFieldHeadString
	case opBool:
		return opStructFieldHeadBool
	}
	return opStructFieldHead
}

func (e *Encoder) typeToFieldType(op opType) opType {
	switch op {
	case opInt:
		return opStructFieldInt
	case opInt8:
		return opStructFieldInt8
	case opInt16:
		return opStructFieldInt16
	case opInt32:
		return opStructFieldInt32
	case opInt64:
		return opStructFieldInt64
	case opUint:
		return opStructFieldUint
	case opUint8:
		return opStructFieldUint8
	case opUint16:
		return opStructFieldUint16
	case opUint32:
		return opStructFieldUint32
	case opUint64:
		return opStructFieldUint64
	case opFloat32:
		return opStructFieldFloat32
	case opFloat64:
		return opStructFieldFloat64
	case opString:
		return opStructFieldString
	case opBool:
		return opStructFieldBool
	}
	return opStructField
}

func (e *Encoder) optimizeStructHeader(op opType, isOmitEmpty, withIndent bool) opType {
	headType := e.typeToHeaderType(op)
	if isOmitEmpty {
		headType = headType.headToOmitEmptyHead()
	}
	if withIndent {
		return headType.toIndent()
	}
	return headType
}

func (e *Encoder) optimizeStructField(op opType, isOmitEmpty, withIndent bool) opType {
	fieldType := e.typeToFieldType(op)
	if isOmitEmpty {
		fieldType = fieldType.fieldToOmitEmptyField()
	}
	if withIndent {
		return fieldType.toIndent()
	}
	return fieldType
}

func (e *Encoder) recursiveCode(typ *rtype, code *compiledCode) *opcode {
	return (*opcode)(unsafe.Pointer(&recursiveCode{
		opcodeHeader: &opcodeHeader{
			op:     opStructFieldRecursive,
			typ:    typ,
			indent: e.indent,
			next:   newEndOp(e.indent),
		},
		jmp: code,
	}))
}

func (e *Encoder) compiledCode(typ *rtype, withIndent bool) *opcode {
	typeptr := uintptr(unsafe.Pointer(typ))
	if withIndent {
		if compiledCode, exists := e.structTypeToCompiledIndentCode[typeptr]; exists {
			return e.recursiveCode(typ, compiledCode)
		}
	} else {
		if compiledCode, exists := e.structTypeToCompiledCode[typeptr]; exists {
			return e.recursiveCode(typ, compiledCode)
		}
	}
	return nil
}

func (e *Encoder) keyNameAndOmitEmptyFromField(field reflect.StructField) (string, bool) {
	keyName := field.Name
	tag := e.getTag(field)
	opts := strings.Split(tag, ",")
	if len(opts) > 0 {
		if opts[0] != "" {
			keyName = opts[0]
		}
	}
	isOmitEmpty := false
	if len(opts) > 1 {
		isOmitEmpty = opts[1] == "omitempty"
	}
	return keyName, isOmitEmpty
}

func (e *Encoder) structHeader(fieldCode *structFieldCode, valueCode *opcode, isOmitEmpty, withIndent bool) *opcode {
	fieldCode.indent--
	op := e.optimizeStructHeader(valueCode.op, isOmitEmpty, withIndent)
	fieldCode.op = op
	switch op {
	case opStructFieldHead,
		opStructFieldHeadOmitEmpty,
		opStructFieldHeadIndent,
		opStructFieldHeadOmitEmptyIndent:
		return valueCode.beforeLastCode()
	}
	return (*opcode)(unsafe.Pointer(fieldCode))
}

func (e *Encoder) structField(fieldCode *structFieldCode, valueCode *opcode, isOmitEmpty, withIndent bool) *opcode {
	code := (*opcode)(unsafe.Pointer(fieldCode))
	op := e.optimizeStructField(valueCode.op, isOmitEmpty, withIndent)
	fieldCode.op = op
	switch op {
	case opStructField,
		opStructFieldOmitEmpty,
		opStructFieldIndent,
		opStructFieldOmitEmptyIndent:
		return valueCode.beforeLastCode()
	}
	return code
}
func (e *Encoder) compileStruct(typ *rtype, root, withIndent bool) (*opcode, error) {
	if code := e.compiledCode(typ, withIndent); code != nil {
		return code, nil
	}
	typeptr := uintptr(unsafe.Pointer(typ))
	compiled := &compiledCode{}
	if withIndent {
		e.structTypeToCompiledIndentCode[typeptr] = compiled
	} else {
		e.structTypeToCompiledCode[typeptr] = compiled
	}
	// header => code => structField => code => end
	//                        ^          |
	//                        |__________|
	fieldNum := typ.NumField()
	fieldIdx := 0
	var (
		head      *structFieldCode
		code      *opcode
		prevField *structFieldCode
	)
	e.indent++
	for i := 0; i < fieldNum; i++ {
		field := typ.Field(i)
		if e.isIgnoredStructField(field) {
			continue
		}
		keyName, isOmitEmpty := e.keyNameAndOmitEmptyFromField(field)
		fieldType := type2rtype(field.Type)
		valueCode, err := e.compile(fieldType, false, withIndent)
		if err != nil {
			return nil, err
		}
		if field.Anonymous {
			f := valueCode.toStructFieldCode()
			for {
				f.op = f.op.headToAnonymousHead()
				if f.op == opStructEnd {
					f.op = opStructAnonymousEnd
				}
				if f.nextField == nil {
					break
				}
				f = f.nextField.toStructFieldCode()
			}
		}
		key := fmt.Sprintf(`"%s":`, keyName)
		fieldCode := &structFieldCode{
			opcodeHeader: &opcodeHeader{
				typ:    fieldType,
				next:   valueCode,
				indent: e.indent,
			},
			anonymousKey: field.Anonymous,
			key:          []byte(key),
			offset:       field.Offset,
		}
		if fieldIdx == 0 {
			code = e.structHeader(fieldCode, valueCode, isOmitEmpty, withIndent)
			head = fieldCode
			prevField = fieldCode
		} else {
			fcode := (*opcode)(unsafe.Pointer(fieldCode))
			code.next = fcode
			code = e.structField(fieldCode, valueCode, isOmitEmpty, withIndent)
			prevField.nextField = fcode
			prevField = fieldCode
		}
		fieldIdx++
	}
	e.indent--

	structEndCode := (*opcode)(unsafe.Pointer(&structFieldCode{
		opcodeHeader: &opcodeHeader{
			op:     opStructEnd,
			typ:    nil,
			indent: e.indent,
		},
	}))
	structEndCode.next = newEndOp(e.indent)
	if withIndent {
		structEndCode.op = opStructEndIndent
	}

	if prevField != nil && prevField.nextField == nil {
		prevField.nextField = structEndCode
	}

	// no struct field
	if head == nil {
		head = &structFieldCode{
			opcodeHeader: &opcodeHeader{
				op:     opStructFieldHead,
				typ:    typ,
				indent: e.indent,
			},
			nextField: structEndCode,
		}
		if withIndent {
			head.op = opStructFieldHeadIndent
		}
		code = (*opcode)(unsafe.Pointer(head))
	}
	head.end = structEndCode
	code.next = structEndCode
	ret := (*opcode)(unsafe.Pointer(head))
	compiled.code = ret
	return ret, nil
}
