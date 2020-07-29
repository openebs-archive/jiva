package main

import (
	"reflect"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

func verify(msg string, x, y interface{}) {
	if reflect.TypeOf(x) != reflect.TypeOf(y) {
		logrus.Errorf("Type Mismatch")
	}
	if x != y {
		debug.PrintStack()
		logrus.Fatalf("%v failed actual: %v desired: %v", msg, x, y)
	}
}
