package udp

// Команды согласно docs/binary_api.md
const (
	BP_CMD_DISCOVERY          uint8 = 0x00 // Discovery Request
	BP_CMD_EVENT_NOTIFICATION uint8 = 0x05 // Уведомление о событии
	BP_CMD_ACK                uint8 = 0xFF
)

type BinaryDiscoveryPacket struct {
	Cmd      uint8
	SN       [32]byte
	Name     [64]byte
	Object   [64]byte
	IP       [4]byte
	Port     uint16
	UID      uint32
	Version  [10]byte
	GitHash  [10]byte
	Revision [10]byte
	Vendor   [32]byte
	Model    [32]byte
}

const (
	N_COILS_PER_SIDE = 6
	N_COIL_SIDES     = 2
)

type ClassificationResult struct {
	Type   uint32
	Class  uint32
	Object uint32
}

type DetectorStatus struct {
	State          uint32
	In             uint32
	Out            uint32
	Inside         uint32
	Speed          float32
	CalibTimeout   uint32
	Level          uint32
	Lights         uint32
	Classification ClassificationResult
	Metal          struct {
		Alarms    uint32
		AlarmsIn  uint32
		AlarmsOut uint32
	}
}

type ZoneConfig struct {
	ZonesH uint32
	ZonesV uint32
	Total  uint32
}

type DetectorZones struct {
	Config ZoneConfig
	Alarm  [N_COILS_PER_SIDE][N_COIL_SIDES]ClassificationResult
	Level  [N_COILS_PER_SIDE][N_COIL_SIDES]uint8
	Cnt    [N_COILS_PER_SIDE][N_COIL_SIDES]uint32
}

type BinaryEventPacket struct {
	Cmd    uint8
	TS     uint32
	Status DetectorStatus
	Zones  DetectorZones
}
