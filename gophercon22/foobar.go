package main

// Foo is a custom structure our server writes and our client reads
type Foo struct {
	Foo int `json:"foo"`
}

// Bar is a custom structure our server writes and our client reads
type Bar struct {
	Bar string `json:"bar"`
}
