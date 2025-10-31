package singleton

import "fmt"

type Singleton struct {
	property string
}

var instance *Singleton

func init() {
	instance = &Singleton{property: "hello, design pattern!"}
}

func GetInstance() *Singleton {
	return instance
}

func (ins *Singleton) Do() {
	fmt.Println(instance.property)
}
