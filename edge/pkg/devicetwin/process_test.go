/*
Copyright 2019 The KubeEdge Authors.

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

package devicetwin

import (
	"encoding/base64"
	"errors"
	"reflect"
	"testing"

	"github.com/beego/beego/orm"
	"github.com/golang/mock/gomock"

	"github.com/kubeedge/beehive/pkg/common"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"github.com/kubeedge/beehive/pkg/core/model"
	"github.com/kubeedge/kubeedge/edge/pkg/devicetwin/dtclient"
	"github.com/kubeedge/kubeedge/edge/pkg/devicetwin/dtcommon"
	"github.com/kubeedge/kubeedge/edge/pkg/devicetwin/dtcontext"
	"github.com/kubeedge/kubeedge/edge/pkg/devicetwin/dtmodule"
	"github.com/kubeedge/kubeedge/edge/pkg/devicetwin/dttype"
	"github.com/kubeedge/kubeedge/edge/pkg/devicetwin/testutil"
	"github.com/kubeedge/kubeedge/pkg/testtools"
)

type CasesDevice []struct {
	name                  string
	context               *dtcontext.DTContext
	deviceID              string
	wantErr               error
	filterReturn          orm.QuerySeter
	queryTableReturn      orm.QuerySeter
	allReturnIntDevice    int64
	allReturnErrDevice    error
	allReturnIntAttribute int64
	allReturnErrAttribute error
	allReturnIntTwin      int64
	allReturnErrTwin      error
	queryTableMockTimes   int
	filterMockTimes       int
	deviceMockTimes       int
	attributeMockTimes    int
	twinMockTimes         int
}

// createFakeDevice() is function to create fake device.
func createFakeDevice() *[]dtclient.Device {
	fakeDevice := new([]dtclient.Device)
	fakeDeviceArray := make([]dtclient.Device, 1)
	fakeDeviceArray[0] = dtclient.Device{ID: "Test"}
	fakeDevice = &fakeDeviceArray
	return fakeDevice
}

// createFakeAttribute() is function to create fake device attribute.
func createFakeDeviceAttribute() *[]dtclient.DeviceAttr {
	fakeDeviceAttr := new([]dtclient.DeviceAttr)
	fakeDeviceAttrArray := make([]dtclient.DeviceAttr, 1)
	fakeDeviceAttrArray[0] = dtclient.DeviceAttr{DeviceID: "Test"}
	fakeDeviceAttr = &fakeDeviceAttrArray
	return fakeDeviceAttr
}

// createFakeDeviceTwin() is function to create fake devicetwin.
func createFakeDeviceTwin() *[]dtclient.DeviceTwin {
	fakeDeviceTwin := new([]dtclient.DeviceTwin)
	fakeDeviceTwinArray := make([]dtclient.DeviceTwin, 1)
	fakeDeviceTwinArray[0] = dtclient.DeviceTwin{DeviceID: "Test"}
	fakeDeviceTwin = &fakeDeviceTwinArray
	return fakeDeviceTwin
}

// TestRegisterDTModule is function to test RegisterDTmodule().
func TestRegisterDTModule(t *testing.T) {
	beehiveContext.InitContext([]string{common.MsgCtxTypeChannel})
	dtContexts, _ := dtcontext.InitDTContext()
	var moduleRegistered bool
	dtc := &DeviceTwin{
		HeartBeatToModule: make(map[string]chan interface{}),
		DTContexts:        dtContexts,
		DTModules:         make(map[string]dtmodule.DTModule),
	}
	tests := []struct {
		name       string
		moduleName string
	}{
		{
			name:       "MemModule",
			moduleName: "MemModule",
		},
		{
			name:       "TwinModule",
			moduleName: "TwinModule",
		},
		{
			name:       "CommModule",
			moduleName: "CommModule",
		},
		{
			name:       "DeviceModule",
			moduleName: "DeviceModule",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dtc.RegisterDTModule(tt.moduleName)
			moduleRegistered = false
			for _, name := range dtc.DTModules {
				if name.Name == tt.moduleName {
					moduleRegistered = true
					break
				}
			}
			if !moduleRegistered {
				t.Errorf("RegisterDTModule failed to register the module %v", tt.moduleName)
			}
		})
	}
}

// TestDTController_distributeMsg is function to test distributeMsg().
func TestDTController_distributeMsg(t *testing.T) {
	beehiveContext.InitContext([]string{common.MsgCtxTypeChannel})
	dtContexts, _ := dtcontext.InitDTContext()
	dtc := &DeviceTwin{
		HeartBeatToModule: make(map[string]chan interface{}),
		DTModules:         make(map[string]dtmodule.DTModule),
		DTContexts:        dtContexts,
	}

	content := testutil.GenerateAddDevicePalyloadMsg(t)

	var msg = &model.Message{
		Header: model.MessageHeader{
			ParentID: DeviceTwinModuleName,
		},
		Content: string(content),
		Router: model.MessageRoute{
			Source:   "edgemgr",
			Resource: "membership/detail",
		},
	}
	tests := []struct {
		name    string
		message interface{}
		wantErr error
	}{
		{
			//Failure Case
			name:    "distributeMsgTest-NilMessage",
			message: "",
			wantErr: errors.New("distribute message, msg is nil"),
		},
		{
			//Failure Case
			name: "distributeMsgTest-ClassifyMsgFail",
			message: model.Message{
				Router: model.MessageRoute{
					Source:   "bus",
					Resource: "membership/detail",
				},
			},
			wantErr: errors.New("not found action"),
		},
		{
			//Failure Case
			name:    "distributeMsgTest-ActualMessage-NoChanel",
			message: *msg,
			wantErr: errors.New("Not found chan to communicate"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := dtc.distributeMsg(tt.message); !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("DTController.distributeMsg() error = %v, wantError %v", err, tt.wantErr)
			}
		})
	}

	//Successful Case
	dh := make(chan interface{}, 1)
	ch := make(chan interface{}, 1)
	mh := make(chan interface{}, 1)
	deh := make(chan interface{}, 1)
	th := make(chan interface{}, 1)
	dtc.DTContexts.CommChan["DeviceStateUpdate"] = dh
	dtc.DTContexts.CommChan["CommModule"] = ch
	dtc.DTContexts.CommChan["MemModule"] = mh
	dtc.DTContexts.CommChan["DeviceModule"] = deh
	dtc.DTContexts.CommChan["TwinModule"] = th
	name := "distributeMsgTest-ActualMessage-Success"
	t.Run(name, func(t *testing.T) {
		if err := dtc.distributeMsg(*msg); !reflect.DeepEqual(err, nil) {
			t.Errorf("DTController.distributeMsg() error = %v, wantError %v", err, nil)
		}
	})
}

// TestSyncSqlite is function to test SyncSqlite().
func TestSyncSqlite(t *testing.T) {
	beehiveContext.InitContext([]string{common.MsgCtxTypeChannel})

	ormerMock, querySeterMock := testtools.InitOrmerMock(t)

	dtContexts, _ := dtcontext.InitDTContext()
	// fakeDevice is used to set the argument of All function
	fakeDevice := createFakeDevice()
	// fakeDeviceAttr is used to set the argument of All function
	fakeDeviceAttr := createFakeDeviceAttribute()
	// fakeDeviceTwin is used to set the argument of All function
	fakeDeviceTwin := createFakeDeviceTwin()
	tests := CasesDevice{
		{
			//Failure Case
			name:                  "SyncSqliteTest-QuerySqliteFailed",
			context:               dtContexts,
			wantErr:               errors.New("Query sqlite failed while syncing sqlite"),
			filterReturn:          querySeterMock,
			queryTableReturn:      querySeterMock,
			allReturnIntDevice:    int64(0),
			allReturnErrDevice:    errors.New("Query sqlite failed while syncing sqlite"),
			allReturnIntAttribute: int64(0),
			allReturnErrAttribute: nil,
			allReturnIntTwin:      int64(0),
			allReturnErrTwin:      nil,
			queryTableMockTimes:   int(1),
			filterMockTimes:       int(0),
			deviceMockTimes:       int(1),
			attributeMockTimes:    int(0),
			twinMockTimes:         int(0),
		},
		{
			//Success Case
			name:                  "SyncSqliteTest-QuerySqliteSuccess",
			context:               dtContexts,
			wantErr:               nil,
			filterReturn:          querySeterMock,
			queryTableReturn:      querySeterMock,
			allReturnIntDevice:    int64(1),
			allReturnErrDevice:    nil,
			allReturnIntAttribute: int64(1),
			allReturnErrAttribute: nil,
			allReturnIntTwin:      int64(1),
			allReturnErrTwin:      nil,
			queryTableMockTimes:   int(4),
			filterMockTimes:       int(3),
			deviceMockTimes:       int(2),
			attributeMockTimes:    int(1),
			twinMockTimes:         int(1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDevice).Return(test.allReturnIntDevice, test.allReturnErrDevice).Times(test.deviceMockTimes)
			querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDeviceAttr).Return(test.allReturnIntAttribute, test.allReturnErrAttribute).Times(test.attributeMockTimes)
			querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDeviceTwin).Return(test.allReturnIntTwin, test.allReturnErrTwin).Times(test.twinMockTimes)
			querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(test.filterReturn).Times(test.filterMockTimes)
			ormerMock.EXPECT().QueryTable(gomock.Any()).Return(test.queryTableReturn).Times(test.queryTableMockTimes)
			if err := SyncSqlite(test.context); !reflect.DeepEqual(err, test.wantErr) {
				t.Errorf("SyncSqlite() error = %v, wantError %v", err, test.wantErr)
			}
		})
	}
}

// TestSyncDeviceFromSqlite is function to test SyncDeviceFromSqlite().
func TestSyncDeviceFromSqlite(t *testing.T) {
	beehiveContext.InitContext([]string{common.MsgCtxTypeChannel})
	ormerMock, querySeterMock := testtools.InitOrmerMock(t)

	dtContext, _ := dtcontext.InitDTContext()
	// fakeDevice is used to set the argument of All function
	fakeDevice := createFakeDevice()
	// fakeDeviceAttr is used to set the argument of All function
	fakeDeviceAttr := createFakeDeviceAttribute()
	// fakeDeviceTwin is used to set the argument of All function
	fakeDeviceTwin := createFakeDeviceTwin()
	tests := CasesDevice{
		{
			//Failure Case
			name:                  "TestSyncDeviceFromSqlite-QueryDeviceFailure",
			context:               dtContext,
			deviceID:              "DeviceA",
			wantErr:               errors.New("Query Device Failed"),
			filterReturn:          querySeterMock,
			queryTableReturn:      querySeterMock,
			allReturnIntDevice:    int64(0),
			allReturnErrDevice:    errors.New("Query Device Failed"),
			allReturnIntAttribute: int64(0),
			allReturnErrAttribute: nil,
			allReturnIntTwin:      int64(0),
			allReturnErrTwin:      nil,
			queryTableMockTimes:   int(1),
			filterMockTimes:       int(1),
			deviceMockTimes:       int(1),
			attributeMockTimes:    int(0),
			twinMockTimes:         int(0),
		},
		{
			//Failure Case
			name:                  "TestSyncDeviceFromSqlite-QueryDeviceAttributeFailed",
			context:               dtContext,
			deviceID:              "DeviceB",
			wantErr:               errors.New("query device attr failed"),
			filterReturn:          querySeterMock,
			queryTableReturn:      querySeterMock,
			allReturnIntDevice:    int64(1),
			allReturnErrDevice:    nil,
			allReturnIntAttribute: int64(0),
			allReturnErrAttribute: errors.New("query device attr failed"),
			allReturnIntTwin:      int64(0),
			allReturnErrTwin:      nil,
			queryTableMockTimes:   int(2),
			filterMockTimes:       int(2),
			deviceMockTimes:       int(1),
			attributeMockTimes:    int(1),
			twinMockTimes:         int(0),
		},
		{
			//Failure Case
			name:                  "TestSyncDeviceFromSqlite-QueryDeviceTwinFailed",
			context:               dtContext,
			deviceID:              "DeviceC",
			wantErr:               errors.New("query device twin failed"),
			filterReturn:          querySeterMock,
			queryTableReturn:      querySeterMock,
			allReturnIntDevice:    int64(1),
			allReturnErrDevice:    nil,
			allReturnIntAttribute: int64(1),
			allReturnErrAttribute: nil,
			allReturnIntTwin:      int64(0),
			allReturnErrTwin:      errors.New("query device twin failed"),
			queryTableMockTimes:   int(3),
			filterMockTimes:       int(3),
			deviceMockTimes:       int(1),
			attributeMockTimes:    int(1),
			twinMockTimes:         int(1),
		},
		{
			//Success Case
			name:                  "TestSyncDeviceFromSqlite-SuccessCase",
			context:               dtContext,
			deviceID:              "DeviceD",
			wantErr:               nil,
			filterReturn:          querySeterMock,
			queryTableReturn:      querySeterMock,
			allReturnIntDevice:    int64(1),
			allReturnErrDevice:    nil,
			allReturnIntAttribute: int64(1),
			allReturnErrAttribute: nil,
			allReturnIntTwin:      int64(1),
			allReturnErrTwin:      nil,
			queryTableMockTimes:   int(3),
			filterMockTimes:       int(3),
			deviceMockTimes:       int(1),
			attributeMockTimes:    int(1),
			twinMockTimes:         int(1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDevice).Return(test.allReturnIntDevice, test.allReturnErrDevice).Times(test.deviceMockTimes)
			querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDeviceAttr).Return(test.allReturnIntAttribute, test.allReturnErrAttribute).Times(test.attributeMockTimes)
			querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDeviceTwin).Return(test.allReturnIntTwin, test.allReturnErrTwin).Times(test.twinMockTimes)
			querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(test.filterReturn).Times(test.filterMockTimes)
			ormerMock.EXPECT().QueryTable(gomock.Any()).Return(test.queryTableReturn).Times(test.queryTableMockTimes)
			if err := SyncDeviceFromSqlite(test.context, test.deviceID); !reflect.DeepEqual(err, test.wantErr) {
				t.Errorf("SyncDeviceFromSqlite() error = %v, wantError %v", err, test.wantErr)
			}
		})
	}
}

// Test_classifyMsg is function to test classifyMsg().
func Test_classifyMsg(t *testing.T) {
	//Encoded resource with LifeCycleConnectETPrefix
	connectTopic := dtcommon.LifeCycleConnectETPrefix + "testtopic"
	encodedConnectTopicResource := base64.URLEncoding.EncodeToString([]byte(connectTopic))
	//Encoded resource with LifeCycleDisconnectETPrefix
	disconnectTopic := dtcommon.LifeCycleDisconnectETPrefix + "testtopic"
	encodedDisconnectResource := base64.URLEncoding.EncodeToString([]byte(disconnectTopic))
	//Encoded resource with other Prefix
	otherTopic := "/membership/detail/result"
	otherEncodedTopic := base64.URLEncoding.EncodeToString([]byte(otherTopic))
	//Encoded eventbus resource
	eventbusTopic := "$hw/events/device/+/state/update"
	eventbusResource := base64.URLEncoding.EncodeToString([]byte(eventbusTopic))

	content := testutil.GenerateAddDevicePalyloadMsg(t)
	tests := []struct {
		name     string
		message  *dttype.DTMessage
		wantBool bool
	}{
		{
			//Failure Case
			name: "classifyMsgTest-UnencodedMessageResource",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "bus",
						Resource: "membership/detail",
					},
				},
			},
			wantBool: false,
		},
		{
			//Success Case
			name: "classifyMsgTest-Source:bus-Prefix:LifeCycleConnectETPrefix",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "bus",
						Resource: encodedConnectTopicResource,
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Success Case
			name: "classifyMsgTest-Source:bus-Prefix:LifeCycleDisconnectETPrefix",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "bus",
						Resource: encodedDisconnectResource,
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Failure Case
			name: "classifyMessageTest-Source:bus-Prefix:OtherPrefix",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "bus",
						Resource: otherEncodedTopic,
					},
					Content: string(content),
				},
			},
			wantBool: false,
		},
		{
			//Success Case
			name: "classifyMessageTest-Source:bus-Resource:eventbus",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "bus",
						Resource: eventbusResource,
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Success Case
			name: "classifyMessageTest-Source:edgemgr-Resource:membership/detail",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "edgemgr",
						Resource: "membership/detail",
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Success Case
			name: "classifyMessageTest-Source:edgemgr-Resource:membership",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "edgemgr",
						Resource: "membership",
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Success Case
			name: "classifyMessageTest-Source:edgemgr-Resourcetwin:cloud_updated",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "edgemgr",
						Resource: "twin/cloud_updated",
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Success Case
			name: "classifyMessageTest-Source:edgemgr-Resource:device/updated-Operation:updated",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:    "edgemgr",
						Resource:  "device/updated",
						Operation: "updated",
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Failure Case
			name: "calssifyMessageTest-Source:edgemgr-no resource and operation",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source: "edgemgr",
					},
					Content: string(content),
				},
			},
			wantBool: false,
		},
		{
			//Success Case
			name: "classifyMessageTest-Source:edgehub-Resource:node/connection",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "edgehub",
						Resource: "node/connection",
					},
					Content: string(content),
				},
			},
			wantBool: true,
		},
		{
			//Failure Case
			name: "classifyMessageTest-Source:edgehub-Resource:node",
			message: &dttype.DTMessage{
				Msg: &model.Message{
					Router: model.MessageRoute{
						Source:   "edgehub",
						Resource: "node",
					},
					Content: string(content),
				},
			},
			wantBool: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyMsg(tt.message); got != tt.wantBool {
				t.Errorf("classifyMsg() = %v, wantError %v", got, tt.wantBool)
			}
		})
	}
}
