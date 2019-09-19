/*
Copyright 2017 The GoStor Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package pool provides memory pool for buffer.
package pool

import "sync"

var bytePool sync.Pool = sync.Pool{}

func NewBuffer(size int) []byte {
	bytePool.New = func() interface{} {
		return make([]byte, size)
	}

	return bytePool.Get().([]byte)
}

func ReleaseBuffer(b []byte) {
	bytePool.Put(b)
}
