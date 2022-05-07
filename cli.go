// https://github.com/lni/dragonboat-example/blob/79f3d372190ee23705517f193dfe7cc839b34a14/ondisk/main.go
// Copyright 2017,2018 Lei Ni (nilei81@gmail.com).
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
	ondisk is an example program for dragonboat's on disk state machine.
*/
package kvs

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/lni/dragonboat/v3"

	"github.com/lni/goutils/syncutil"
)

type RequestType uint64

const (
	Unknown RequestType = iota
	PUT
	GET
	DELETE
)

func ParseCommand(msg string) (RequestType, string, []byte, bool) {
	parts := strings.Split(strings.TrimSpace(msg), " ")
	if len(parts) == 0 {
		return PUT, "", []byte(""), false
	}
	switch parts[0] {
	case "put":
		if len(parts) != 3 {
			return PUT, "", []byte(""), false
		}
		return PUT, parts[1], []byte(parts[2]), true
	case "get":
		if len(parts) != 2 {
			return GET, "", []byte(""), false
		}
		return GET, parts[1], []byte(""), true
	case "delete":
		if len(parts) != 2 {
			return DELETE, "", []byte(""), false
		}
		return DELETE, parts[1], []byte(""), true
	}

	return Unknown, "", []byte(""), false
}

func printUsage() {
	fmt.Fprintf(os.Stdout, "Usage - \n")
	fmt.Fprintf(os.Stdout, "put key value\n")
	fmt.Fprintf(os.Stdout, "get key\n")
}
func Console(raftStopper *syncutil.Stopper, ch chan string, nh *dragonboat.NodeHost) {
	consoleStopper := syncutil.NewStopper()
	consoleStopper.RunWorker(func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			s, err := reader.ReadString('\n')
			if err != nil {
				close(ch)
				return
			}
			if s == "exit\n" {
				raftStopper.Stop()
				nh.Stop()
				return
			}
			ch <- s
		}
	})
	printUsage()
}
