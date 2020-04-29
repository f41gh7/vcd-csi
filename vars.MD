# Auto Generated vars for package conf 
## updated at Wed Apr 29 22:50:47 UTC 2020 


| varible name | variable default value | variable required | variable description |
| --- | --- | --- | --- |
| VCSI_HTTPLISTEN | 0.0.0.0:8155 | false | health check web url |
| VCSI_UNIXSOCKET | unix:///var/run/vcd-csi.sock | false | - |
| VCSI_CONTROLLERNAME | vcd.csi.fght.net | false | - |
| VCSI_NODENAME | local | false | node name, usefull for daemonset |
| VCSI_NODEVDC | vcd-1 | false | name of vcd, where node located, must be set for d |
| VCSI_VDCS | - | true | names of VDCs comma separated, client would be cre |
| VCSI_DEFAULTSUBBUSTYPE | VirtualSCSI | false | enum VirtualSCSI,lsilogicsas,lsilogic,buslogic, on |
| VCSI_DEFAULTBUSTYPE | 6 | false | //5 - IDE, 6 - SCSI, 20 - SATA, only 6 is supporte |
| VCSI_DEFAULTBUSNUM | 3 | false | vm controller num enum: 0,1,2,3, i recommend to us |
| VCSI_CLOUDCREDENTAILS_USER | - | true | username |
| VCSI_CLOUDCREDENTAILS_PASSWORD | - | true | passsword |
| VCSI_CLOUDCREDENTAILS_ORG | - | true | some-org |
| VCSI_CLOUDCREDENTAILS_HREF | - | true | https://vcd.cloud/api |
| VCSI_CLOUDCREDENTAILS_INSECURE | false | false | - |
