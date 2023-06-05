package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	protoMessage := os.Args[1]
	protoFiles := os.Args[2:]

	reg := new(protoregistry.Files)

	err := registerProtoFiles(reg, protoFiles...)
	if err != nil {
		log.Fatalln("Failed to register proto files:", err)
	}

	md, err := findMessageDescriptor(reg, protoreflect.FullName(protoMessage))
	if err != nil {
		log.Fatalln("Failed to find descriptor:", err)
	}

	m := GenerateRandomMessage(protoreflect.MessageDescriptor(md))

	err = writeMessage(m.Interface(), os.Stdout)
	if err != nil {
		log.Fatalln("Failed to write message:", err)
	}
}

func writeMessage(m protoreflect.ProtoMessage, w io.Writer) error {
	data, err := proto.Marshal(m)
	if err != nil {
		return fmt.Errorf("Failed to marshal proto message: %w", err)
	}

	_, err = w.Write(data)
	return err
}

func registerProtoFiles(reg *protoregistry.Files, files ...string) error {
	var (
		tmpfileDir    string = ""
		tmpfilePrefix string = ""
	)
	tmpfile, err := os.CreateTemp(tmpfileDir, tmpfilePrefix)
	if err != nil {
		log.Fatalln("Failed to create temporary file:", err)
	}
	defer os.Remove(tmpfile.Name())

	opts := []string{
		"--descriptor_set_out=" + tmpfile.Name(),
		"--include_imports",
	}
	cmd := exec.Command(
		"protoc",
		append(opts, files...)...,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	protoFile, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return err
	}

	fds := new(descriptorpb.FileDescriptorSet)
	if err = proto.Unmarshal(protoFile, fds); err != nil {
		return err
	}

	for _, fdp := range fds.GetFile() {
		fd, err := protodesc.NewFile(fdp, reg)
		if err != nil {
			return err
		}

		if err = reg.RegisterFile(fd); err != nil {
			return err
		}
	}

	return nil
}

func findMessageDescriptor(reg *protoregistry.Files, name protoreflect.FullName) (protoreflect.MessageDescriptor, error) {
	var md protoreflect.MessageDescriptor

	reg.RangeFiles(
		func(fd protoreflect.FileDescriptor) bool {
			if strings.Contains(string(name), string(fd.Package())) {
				md = fd.Messages().ByName(name.Name())
				if md != nil {
					return false
				}
			}
			return true
		},
	)
	if md == nil {
		return nil, errors.New("not found")
	}

	return md, nil
}

func GenerateRandomMessage(md protoreflect.MessageDescriptor) protoreflect.Message {
	fmt.Fprintln(os.Stderr, md.Name())
	if md.FullName() == "google.protobuf.Timestamp" {
		return timestamppb.Now().ProtoReflect()
	}

	// we need a `protoreflect.Message` from the `protoreflect.MessageDescriptor` here,
	// and the only way I found to get it is via a `dynamicpb.Message`
	m := dynamicpb.NewMessage(md).New()
	RangeFields(md, SetRandomValueFun(m))
	return m
}

func RangeFields(md protoreflect.MessageDescriptor, fun func(protoreflect.FieldDescriptor) bool) {
	for i := 0; i < md.Fields().Len(); i++ {
		fun(md.Fields().Get(i))
	}
}

func SetRandomValueFun(m protoreflect.Message) func(protoreflect.FieldDescriptor) bool {
	return func(fd protoreflect.FieldDescriptor) bool {
		if fd.IsList() {
			m.Mutable(fd).List().Append(GenerateRandomValue(fd))
		} else {
			m.Set(fd, GenerateRandomValue(fd))
		}

		return true
	}
}

func GenerateRandomValue(fd protoreflect.FieldDescriptor) protoreflect.Value {
	switch kind := fd.Kind(); kind {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(true)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOf(int32(42))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOf(int64(1337))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOf(uint32(42))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOf(uint64(1337))
	case protoreflect.FloatKind:
		return protoreflect.ValueOf(float32(2.71828))
	case protoreflect.DoubleKind:
		return protoreflect.ValueOf(float64(3.141592653))
	case protoreflect.StringKind:
		return protoreflect.ValueOf("Lorem ipsum")
	case protoreflect.BytesKind:
		return protoreflect.ValueOf([]byte{0xc0, 0xff, 0xee})
	case protoreflect.EnumKind:
		return protoreflect.ValueOfEnum(fd.Enum().Values().Get(0).Number())
	case protoreflect.MessageKind:
		nd := fd.Message()
		n := GenerateRandomMessage(nd)
		return protoreflect.ValueOfMessage(n)
	}
	return protoreflect.Value{}
}
