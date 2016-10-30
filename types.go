package main

type RecordA struct {
	Ttl    *uint32  `json:"-"`
	Answer []string `json:"answer"`
}

type RecordCname struct {
	Ttl    *uint32 `json:"-"`
	Answer string  `json:"answer"`
}

type RecordPtr struct {
	Ttl    *uint32 `json:"-"`
	Answer string  `json:"answer"`
}

type RecordTxt struct {
	Ttl    *uint32  `json:"-"`
	Answer []string `json:"answer"`
}

type ClientAnswers struct {
	Search        []string               `json:"search"`
	Recurse       []string               `json:"recurse"`
	Authoritative []string               `json:"authorative"`
	A             map[string]RecordA     `json:"a"`
	Cname         map[string]RecordCname `json:"cname"`
	Ptr           map[string]RecordPtr   `json:"-"`
	Txt           map[string]RecordTxt   `json:"-"`
}

type Answers map[string]ClientAnswers
