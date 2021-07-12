/*
 * Copyright (c) 2020   f41gh7
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package locker


import "sync"

type Locker interface {
	Put(string)
	Pop (string)bool
	IsLocked(string)bool
}

type SimpleLock struct {
	storage map[string]bool
	m *sync.Mutex
}

func NewSimpleLock ()(*SimpleLock,error){
	return &SimpleLock{
		storage: map[string]bool{},
		m:       &sync.Mutex{},
	},nil
}

func (sl *SimpleLock)Put(key string){
	sl.storage[key] = true
}

func (sl *SimpleLock)IsLocked(key string)bool{
	sl.m.Lock()
	defer sl.m.Unlock()
	if _,ok := sl.storage[key];ok {
		delete(sl.storage,key)
		return true
	}
	return false
}


func (sl *SimpleLock)Pop(key string)bool{
	sl.m.Lock()
	defer sl.m.Unlock()
	if _,ok := sl.storage[key];ok {
		delete(sl.storage,key)
		return true
	}
	return false
}
