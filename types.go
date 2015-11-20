package main

type RecordA struct {
	Ttl    *uint32
	Answer []string
}

type RecordCname struct {
	Ttl    *uint32
	Answer string
}

type RecordPtr struct {
	Ttl    *uint32
	Answer string
}

type RecordTxt struct {
	Ttl    *uint32
	Answer []string
}

type ClientAnswers struct {
	Recurse []string
	A       map[string]RecordA
	Cname   map[string]RecordCname
	Ptr     map[string]RecordPtr
	Txt     map[string]RecordTxt
}

type Answers map[string]ClientAnswers
