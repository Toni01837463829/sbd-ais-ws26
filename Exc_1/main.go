package main

import "fmt"


type Person struct {
	Name string
	Age  int
}


func (p Person) Print() {
	fmt.Printf("Hello, World! I am %s, %d years old.\n", p.Name, p.Age)
}


func main() {
	p := Person{Name: "Anton", Age: 21}
	p.Print()
}
