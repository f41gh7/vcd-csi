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

package main

import (
	"context"
	"fmt"
	"github.com/f41gh7/vcd-csi/conf"
	"github.com/f41gh7/vcd-csi/internal/driver"
	"github.com/f41gh7/vcd-csi/internal/locker"
	"github.com/f41gh7/vcd-csi/internal/mount"
	v1 "github.com/f41gh7/vcd-csi/pkg/controller/v1"
	vcd_client "github.com/f41gh7/vcd-csi/pkg/vcd-client"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	Version   string
	BuildDate string
)

func main() {
	fmt.Printf("launching with version: %s, build date :%s\n", Version, BuildDate)
	c, err := conf.NewControllerConfig()
	if err != nil {
		panic(err)
	}

	l := c.GetLogger()
	wg := &sync.WaitGroup{}
	l.Infof("inited config and logger")
	l.Infof("initing client")
	client, err := vcd_client.NewVcdClient(c, l)
	if err != nil {
		l.WithError(err).Errorf("cannot create VCD client")
		os.Exit(1)
	}
	l.Infof("init locker")
	lock,err := locker.NewSimpleLock()
	if err != nil {
		l.WithError(err).Errorf("cannot create locker")
		panic(err)
	}

	mounter := mount.NewMounter(l)
	l.Infof("inited client, lets start driver init")
	csiDriver, err := driver.NewCsiDriver(l, c, client, mounter,lock, wg)
	if err != nil {
		l.WithError(err).Errorf("cannot create driver")
		os.Exit(1)
	}

	l.Infof("driver inited")
	l.Infof("initing controller")

	controller, err := v1.NewCsiController(l, c, csiDriver, wg)
	if err != nil {
		l.WithError(err).Errorf("cannot create csi controller")
		os.Exit(1)
	}

	l.Infof("controller inited, starting")
	//create shutdown channel
	out := make(chan os.Signal)
	signal.Notify(out, syscall.SIGTERM)
	signal.Notify(out, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.TODO())
	wg.Add(1)
	go func() {
		err := controller.Run(ctx)
		if err != nil {
			l.WithError(err).Errorf("error starting controller")
			panic(err)
		}
	}()

	<-out
	l.Infof("received shutdown to main, stopping ")
	cancel()

	wg.Wait()

}
