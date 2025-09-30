package stateful

import (
	"errors"
	"testing"
	"time"

	"github.com/lcx/asura/log"
	"github.com/lcx/asura/net"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// createTestConfig creates a test configuration for stateful layer testing
func createTestConfig() *StatefulConfig {
	return &StatefulConfig{
		TickPeriodMillSec:   20 * 1000, // 20 seconds
		GrHBTimeoutSec:      10,        // 10 seconds
		BroadcastQPS:        10000,     // 10,000 notifications per second
		BroadcastMaxWaitNum: 10,        // Queue up to 10 pending messages
		MaxActorCount:       5000,      // Allow up to 5000 concurrent actors
		MigrateFeatSwitch:   false,     // Migration feature disabled by default
	}
}

// MockActor implements the Actor interface for testing
type MockActor struct {
	ActorBase
	initErr        error
	tickErr        error
	migrateFailErr error
	saveCalled     bool
	exitCalled     bool
	updateInternMS int
}

func (m *MockActor) Init() error {
	return m.initErr
}

func (m *MockActor) OnTick() error {
	return m.tickErr
}

func (m *MockActor) OnGracefulExit() {
	m.exitCalled = true
}

func (m *MockActor) OnMigrateFailRecover() error {
	return m.migrateFailErr
}

func (m *MockActor) Save() {
	m.saveCalled = true
}

func (m *MockActor) ShowChange() (proto.Message, string) {
	return nil, "mock change"
}

func (m *MockActor) EncodeMigrateData() ([]byte, error) {
	return []byte("mock data"), nil
}

func (m *MockActor) GetUpdateInternMS() int {
	return m.updateInternMS
}

func (m *MockActor) Debug() *log.LogEvent {
	return log.Debug()
}

func (m *MockActor) Info() *log.LogEvent {
	return log.Info()
}

func (m *MockActor) Warn() *log.LogEvent {
	return log.Warn()
}

func (m *MockActor) Error() *log.LogEvent {
	return log.Error()
}

func (m *MockActor) Fatal() *log.LogEvent {
	return log.Fatal()
}

func (m *MockActor) IgnoreCheckLevel() bool {
	return false
}

func (m *MockActor) GetAppender() []log.LogAppender {
	return nil
}

func (m *MockActor) AddAppender(appender log.LogAppender) {
}

func (m *MockActor) OnEventEnd(e *log.LogEvent) {
}

func TestNewMsgLayer(t *testing.T) {
	msgMgr := net.NewMessageManager()
	layer, err := NewMsgLayer(nil, msgMgr, nil, nil, nil)
	assert.Error(t, err) // Should return error for nil config
	assert.Nil(t, layer)
}

func TestStatefulMsgLayer_Init(t *testing.T) {
	msgMgr := net.NewMessageManager()
	layer, err := NewMsgLayer(nil, msgMgr, nil, nil, nil)
	assert.Error(t, err) // Should return error for nil config

	// Test with custom config
	customConfig := createTestConfig()
	customConfig.ActorLifeSecond = 3600
	customConfig.TickPeriodMillSec = 1000
	layer, err = NewMsgLayer(customConfig, msgMgr, nil, nil, nil)
	assert.NoError(t, err)

	err = layer.Init()
	assert.NoError(t, err)
}

func TestStatefulMsgLayer_PostPkgLocal_ActorCreationError(t *testing.T) {
	msgMgr := net.NewMessageManager()

	// Add protocol info for testing
	protoInfo := &net.MsgProtoInfo{
		MsgID:        "test.msg",
		MsgLayerType: net.MsgLayerType_Stateful,
	}
	msgMgr.PropInfoMap["test.msg"] = protoInfo

	// Create a mock actor creator that returns error
	actorCreator := func(uid uint64) (Actor, error) {
		return nil, errors.New("creation failed")
	}

	layer, err := NewMsgLayer(createTestConfig(), msgMgr, actorCreator, nil, nil)
	assert.NoError(t, err)
	err = layer.Init()
	assert.NoError(t, err)

	// Test actor creation error by directly calling createActor
	// The createActor method will be called internally when trying to get a non-existent actor
	delivery := &net.DispatcherDelivery{
		TransportDelivery: &net.TransportDelivery{
			Pkg: net.NewTransRecvPkgWithBody(nil, &net.PkgHead{MsgID: "test.msg", DstActorID: 12345}, nil),
		},
		ProtoInfo: protoInfo,
	}

	// This will trigger actor creation through tryGetActorRuntime
	err = layer.DispatchCachedPkg(12345, delivery)
	assert.Error(t, err)
}

func TestActorRuntime_HandlePkg(t *testing.T) {
	msgMgr := net.NewMessageManager()
	layer, err := NewMsgLayer(createTestConfig(), msgMgr, nil, nil, nil)
	assert.NoError(t, err)

	mockActor := &MockActor{}
	runtime := newActorRuntime(layer, 12345, mockActor)

	// Create a mock handle context
	pkg := net.NewTransRecvPkgWithBody(nil, &net.PkgHead{MsgID: "test_msg"}, &net.PkgHead{})

	hCtx := &HandleContext{
		DispatcherDelivery: &net.DispatcherDelivery{
			TransportDelivery: &net.TransportDelivery{
				Pkg: pkg,
			},
			ProtoInfo: &net.MsgProtoInfo{
				MsgID: "test_msg",
				MsgHandle: MsgHandle(func(hCtx *HandleContext, actor Actor, body proto.Message) (proto.Message, int32) {
					return nil, 0
				}),
			},
		},
		actorID: 12345,
		Logger:  mockActor,
	}

	err = runtime.handlePkg(hCtx)
	assert.NoError(t, err)
}

func TestActorRuntime_Tick(t *testing.T) {
	msgMgr := net.NewMessageManager()
	config := createTestConfig()
	config.SaveCategory.FreqSaveSecond = 1
	layer, err := NewMsgLayer(config, msgMgr, nil, nil, nil)
	assert.NoError(t, err)

	mockActor := &MockActor{}
	runtime := newActorRuntime(layer, 12345, mockActor)
	time.Sleep(time.Second*time.Duration(config.SaveCategory.FreqSaveSecond) + time.Microsecond*500)

	// Test tick
	runtime.tick()

	// Verify Save was called
	assert.True(t, mockActor.saveCalled)
}

func TestActorRuntime_ExitActor(t *testing.T) {
	msgMgr := net.NewMessageManager()
	layer, err := NewMsgLayer(createTestConfig(), msgMgr, nil, nil, nil)
	assert.NoError(t, err)

	mockActor := &MockActor{}
	runtime := newActorRuntime(layer, 12345, mockActor)

	// Test exit actor
	runtime.exitActor()

	// Verify context was cancelled
	select {
	case <-runtime.ctx.Done():
		// Context was cancelled as expected
	default:
		t.Error("Context should be cancelled after exitActor")
	}
}

func TestHandleContext(t *testing.T) {
	msgMgr := net.NewMessageManager()
	layer, err := NewMsgLayer(createTestConfig(), msgMgr, nil, nil, nil)
	assert.NoError(t, err)

	// Create a mock handle context
	pkg := net.NewTransRecvPkgWithBody(nil, &net.PkgHead{MsgID: "test_msg"}, &net.PkgHead{})

	hCtx := &HandleContext{
		DispatcherDelivery: &net.DispatcherDelivery{
			TransportDelivery: &net.TransportDelivery{
				Pkg: pkg,
			},
			ProtoInfo: &net.MsgProtoInfo{
				MsgID: "test_msg",
			},
		},
		actorID:  12345,
		Logger:   &MockActor{},
		msgMgr:   msgMgr,
		msgLayer: layer,
	}

	// Test GetActorID
	assert.Equal(t, uint64(12345), hCtx.GetActorID())

	// Test SetActorID
	hCtx.SetActorID(67890)
	assert.Equal(t, uint64(67890), hCtx.GetActorID())
}

func TestActorMgr(t *testing.T) {
	msgMgr := net.NewMessageManager()
	layer, err := NewMsgLayer(createTestConfig(), msgMgr, nil, nil, nil)
	assert.NoError(t, err)

	mockActor := &MockActor{}
	creator := func(uid uint64) (Actor, error) {
		return mockActor, nil
	}

	mgr := newActorMgr(creator)
	mgr.msgLayer = layer

	// Test createActor
	runtime1, err := mgr.createActor(12345)
	assert.NoError(t, err)
	assert.NotNil(t, runtime1)

	// Test getActorRuntime - existing actor
	runtime2, ok := mgr.getActorRuntime(12345)
	assert.True(t, ok)
	assert.Equal(t, runtime1, runtime2)

	// Test getActorRuntime - new actor
	runtime3, ok := mgr.getActorRuntime(67890)
	assert.False(t, ok)
	assert.Nil(t, runtime3)

	// Test getAllActors
	allActors := mgr.getAllActors()
	assert.Len(t, allActors, 1)
	assert.Contains(t, allActors, runtime1)

	// Test getAllActorsAID
	allIDs := mgr.getAllActorsAID()
	assert.Len(t, allIDs, 1)
	assert.Contains(t, allIDs, uint64(12345))

	// Test deleteActor
	mgr.deleteActor(12345)
	time.Sleep(time.Microsecond * 500)
	assert.Equal(t, 0, mgr.actorCount())

	// Test shutdown
	mgr.shutdown()
	assert.Equal(t, 0, mgr.actorCount())
}
