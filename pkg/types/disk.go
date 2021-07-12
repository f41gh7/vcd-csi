/*
 * Copyright (c) 2021   f41gh7
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

package types

// StorageSize - holds size of storage request.
type StorageSize struct {
	b int64
}

func NewStorageSizeFromMb(mb int64)*StorageSize{
	return &StorageSize{
		b: mb/1024/1024,
	}
}
// NewStorageSize - creates storage size from given bytes.
func NewStorageSize(b int64)*StorageSize{
	return &StorageSize{b}
}

// Mb - returns storage size in megabytes.
func (s *StorageSize)Mb()int64{
	return s.b/1024/1024
}

// Bytes - returns storage size in bytes
func (s *StorageSize)Bytes()int64{
	return s.b
}