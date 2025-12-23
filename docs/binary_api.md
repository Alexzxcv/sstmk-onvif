# Binary Protocol

## Commands

| Command ID | Name | Description |
| :--- | :--- | :--- |
| 0x00 | BP_CMD_DISCOVERY | Discovery |
| 0x01 | BP_CMD_GET_CONF | Get configuration |
| 0x02 | BP_CMD_SET_CONF | Set configuration |
| 0x03 | BP_CMD_GET_DETECTOR_STATUS | Get detector status |
| 0x04 | BP_CMD_GET_DETECTOR_ZONES | Get detector zones |
| 0xFF | BP_CMD_ACK | Acknowledge |

## Discovery

### Request

Server sends a UDP packet with command ID `0x00`. Payload can be empty.

### Response

Device responds with `bp_discovery_packet_t`:

| Offset | Size | Type | Name | Description |
| :--- | :--- | :--- | :--- | :--- |
| 0 | 1 | uint8_t | cmd | Command ID (0x05) |
| 1 | 32 | char | sn | Serial Number |
| 33 | 64 | char | name | Device Name |
| 97 | 64 | char | object_name | Object Name |
| 161 | 4 | uint8_t | ip | IP Address |
| 165 | 2 | uint16_t | port | Port |
| 167 | 1 | uint8_t | ver | Config Version |
| 168 | 4 | uint32_t | uid | Unique ID |
