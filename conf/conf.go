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

package conf

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

const prefixVar = "VCSI"

//genvars:true
type ControllerConfig struct {
	HttpListen        string   `default:"0.0.0.0:8155" description:"health check web url"`
	UnixSocket        string   `default:"unix:///var/run/vcd-csi.sock"`
	ControllerName    string   `default:"vcd.csi.fght.net"`
	NodeName          string   `default:"local" description:"node name, usefull for daemonset"`
	NodeVdc           string   `default:"vcd-1" description:"name of vcd, where node located, must be set for daemonset with affinity"`
	Vdcs              []string `description:"names of VDCs comma separated, client would be created for each one" required:"true"`
	DefaultSubBusType string   `default:"VirtualSCSI" description:"enum VirtualSCSI,lsilogicsas,lsilogic,buslogic, only VirtualSCSI supported"`
	DefaultBusType    string   `default:"6" description:"//5 - IDE, 6 - SCSI, 20 - SATA, only 6 is supported atm"`
	DefaultBusNum     int      `default:"3" description:"vm controller num enum: 0,1,2,3, i recommend to use 3"`
	LogLevel          string
	CloudCredentails  struct {
		User     string `required:"true" description:"username"`
		Password string `required:"true" description:"passsword"`
		Org      string `required:"true" description:"some-org"`
		Href     string `required:"true" description:"https://vcd.cloud/api"`
		Insecure bool   `default:"false"`
	}
}

func (cc *ControllerConfig) GetLogger() *logrus.Entry {
	log := logrus.New()
	log.SetLevel(func(lvl string) logrus.Level {
		level, err := logrus.ParseLevel(lvl)
		if err != nil {
			log.Errorf("failed to parse log level: %s, using default INFO", lvl)
			return logrus.InfoLevel
		}
		return level
	}(cc.LogLevel))
	log.ReportCaller = true
	log.SetFormatter(&logrus.JSONFormatter{})

	return logrus.NewEntry(log)
}

func NewControllerConfig() (*ControllerConfig, error) {
	c := &ControllerConfig{}
	err := envconfig.Process(prefixVar, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
