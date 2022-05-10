package webtty

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/opensourceways/gotty/utils"
	"github.com/pkg/errors"
)

var (
	Log      = make([]string, 0)
	Instance string
	Ip       = make([]string, 0)
)

type LogType int

const (
	Login = iota
	Logout
	Operation
)

// WebTTY bridges a PTY slave and its PTY master.
// To support text-based streams and side channel commands such as
// terminal resizing, WebTTY uses an original protocol.
type WebTTY struct {
	// PTY Master, which probably a connection to browser
	masterConn Master
	// PTY Slave
	slave Slave

	windowTitle []byte
	permitWrite bool
	columns     int
	rows        int
	reconnect   int // in seconds
	masterPrefs []byte
	recordLog   bool

	bufferSize int
	writeMutex sync.Mutex
}

// New creates a new instance of WebTTY.
// masterConn is a connection to the PTY master,
// typically it's a websocket connection to a client.
// slave is a PTY slave such as a local command with a PTY.
func New(masterConn Master, bufferSize int, slave Slave, options ...Option) (*WebTTY, error) {
	wt := &WebTTY{
		masterConn: masterConn,
		slave:      slave,

		permitWrite: false,
		columns:     0,
		rows:        0,
		recordLog:   false,

		bufferSize: bufferSize,
	}

	for _, option := range options {
		option(wt)
	}

	return wt, nil
}

// Run starts the main process of the WebTTY.
// This method blocks until the context is canceled.
// Note that the master and slave are left intact even
// after the context is canceled. Closing them is caller's
// responsibility.
// If the connection to one end gets closed, returns ErrSlaveClosed or ErrMasterClosed.
func (wt *WebTTY) Run(ctx context.Context) error {
	var (
		Command = make(chan string)
		Uinput  = make(chan string)
		Message = make(chan string)

		command    string
		commandEnd bool
		message    string
		bel        bool
		bs         bool
		index      = math.MaxInt
	)
	err := wt.sendInitializeMessage()
	if err != nil {
		return errors.Wrapf(err, "failed to send initializing message")
	}

	errs := make(chan error, 2)

	if wt.recordLog {
		go func() {
			for {
				select {
				case s := <-Uinput:
					ipd := strings.Split(s, "-terminal-")
					ps, data := ipd[0], []byte(ipd[1])
					if utils.FilterOutput(data) || data[len(data)-1] == uint8(13) {
						continue
					}
					if !ipIsInSlice(ps) {
						Ip = append(Ip, ps)
						RecordLog(ps, "", "", Login)
						continue
					}
					if len(data) >= 7 && utils.EqualTwoSliceByClear(data[:7]) {
						continue
					}
					if len(data) == 1 && data[0] == uint8(7) {
						bel = true
						continue
					}
					if bel {
						if len(data) == 2 && data[0] == uint8(13) && data[1] == uint8(10) {
							continue
						}
						if !utils.EqualTwoSliceByBack(data) && len(data) > 2 {
							if i := judgeRPosition(data); i != -1 {
								continue
							}
							bel = false
						} else {
							bel = false
						}
					}
					if utils.EqualTwoSliceByBack(data) || (len(data) == 3 && data[0] == uint8(8) && data[1] == uint8(32) && data[2] == uint8(8)) {
						if command != "" {
							command = command[:len(command)-1]
						}
						continue
					}
					if command != "" && !utils.EqualNRForZsh(data) && !utils.EqualTwoSliceByBack(data) {
						if len(data) > 1 && data[0] == uint8(8) {
							command = string(utils.DeleteBs(data))
							continue
						}
						if l := strings.Index(string(data), strings.TrimSpace(command)); l > 0 {
							command = string(data)[l:]
							continue
						}
					}
					if commandEnd {
						if len(data) == 11 && utils.ExistZshOutPut(data) {
							continue
						}
						if len(data) >= 2 && (data[len(data)-2] == uint8(13) && data[len(data)-1] == uint8(10)) {
							if len(data) > 11 && utils.ExistZshOutPut(data[:11]) {
								message += string(data[11:])
							} else {
								message += string(data)
							}
							continue
						} else {
							if i := judgeRPosition(data); i == -1 {
								Message <- message
								initParameter(&message, &command, &index, &commandEnd, &bel, &bs, false)
								continue
							} else {
								if utils.ExistZshOutPut(data[:i]) {
									message += string(data[11:i])
								} else {
									message += string(data[:i])
									if i == 9 {
										message = ""
									}
								}
								Message <- message
								initParameter(&message, &command, &index, &commandEnd, &bel, &bs, false)
								continue
							}
						}
					}
					if utils.EqualNR(data) || utils.EqualNRForZsh(data) || (len(data) > 2 && utils.EqualNR(data[:2])) || utils.ExistZshOutPut(data) {
						Command <- ps + "-terminal-" + command
						initParameter(&message, &command, &index, &commandEnd, &bel, &bs, true)
						if len(data) > 2 && utils.EqualNR(data[:2]) {
							databak := data[2:]
							i := judgeRPosition(databak)
							if i != -1 {
								message = string(databak[:i])
							}
							Message <- message
							initParameter(&message, &command, &index, &commandEnd, &bel, &bs, false)
						}
					} else {
						if i := judgeRPosition(data); i > -1 {
							if i == 0 {
								Command <- ps + "-terminal-" + command
								initParameter(&message, &command, &index, &commandEnd, &bel, &bs, true)
								Message <- message
								initParameter(&message, &command, &index, &commandEnd, &bel, &bs, false)
							}
							continue
						}
						if len(data) == 1 && data[0] == uint8(8) {
							if index == math.MaxInt {
								index = len(command) - 1
							} else {
								if index > 0 {
									index -= 1
								}
							}
							bs = true
							continue
						} else if bs {
							if data[0] == uint8(8) {
								if index > 0 {
									index -= 1
								}
								if len(data) > 5 && utils.ExistBytes(data[1:5]) {
									command = command[:index] + string(utils.DeleteBs(data[5:]))
								}
								continue
							}
							//in the bash command, delete the same letter
							if len(data) == 4 && data[0] == uint8(27) && data[1] == uint8(91) && data[2] == uint8(75) && data[3] == uint8(8) {
								if index > 0 {
									index -= 1
								}
								command = command[:index] + command[index+1:]
								continue
							}
							if utils.ExistBytes(data) {
								index += 1
								continue
							}
							if index > len(command) {
								index = len(command)
							}
							commandBak := command[:index]
							if utils.ExistBytes(data) {
								if len(data) > 3 {
									commandBak += string(data[3])
									data = data[3:]
								}
							}
							result := utils.DeleteBs(data)
							command = commandBak + string(result)
							index += 1
							continue
						}
						result := utils.DeleteBel(data)
						if !bs && len(data) > 1 && data[len(data)-1] == uint8(8) {
							command = command[:len(command)-1]
							if utils.ExistBytes(data) {
								if len(data) > 3 {
									data = data[3:]
								}
							}
							result = utils.DeleteBs(data)
						}
						if !bs && len(data) > 1 && data[0] == uint8(8) {
							result = utils.DeleteBs(result)
							command = string(result)
							continue
						}
						command += string(result)
					}
				}
			}
		}()

		go func() {
			for {
				select {
				case cmd := <-Command:
					output := <-Message
					ipd := strings.Split(cmd, "-terminal-")
					input := ipd[1]
					if len(ipd) < 2 || len(input) == 0 || strings.Contains(input, "1H~") {
						continue
					}
					if utils.ExistVi(input) || input == "clear" {
						output = ""
					}
					fmt.Printf("ps:%s\n命令是:%s\n信息是:%s\n", ipd[0], input, output)
					RecordLog(ipd[0], input, output, Operation)
				}
			}
		}()
	}

	go func() {
		errs <- func() error {
			buffer := make([]byte, wt.bufferSize)
			for {
				n, err := wt.slave.Read(buffer)
				if err != nil {
					return ErrSlaveClosed
				}
				if wt.recordLog {
					go func() {
						Uinput <- wt.masterConn.Ip() + "-terminal-" + string(buffer[:n])
						return
					}()
				}
				time.Sleep(200 * time.Microsecond)
				err = wt.handleSlaveReadEvent(buffer[:n])
				if err != nil {
					return err
				}
			}
		}()
	}()

	go func() {
		errs <- func() error {
			buffer := make([]byte, wt.bufferSize)
			for {
				n, err := wt.masterConn.Read(buffer)
				if err != nil {
					return ErrMasterClosed
				}

				err = wt.handleMasterReadEvent(buffer[:n])
				if err != nil {
					return err
				}
			}
		}()
	}()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errs:
	}

	return err
}

func (wt *WebTTY) sendInitializeMessage() error {
	err := wt.masterWrite(append([]byte{SetWindowTitle}, wt.windowTitle...))
	if err != nil {
		return errors.Wrapf(err, "failed to send window title")
	}

	if wt.reconnect > 0 {
		reconnect, _ := json.Marshal(wt.reconnect)
		err := wt.masterWrite(append([]byte{SetReconnect}, reconnect...))
		if err != nil {
			return errors.Wrapf(err, "failed to set reconnect")
		}
	}

	if wt.masterPrefs != nil {
		err := wt.masterWrite(append([]byte{SetPreferences}, wt.masterPrefs...))
		if err != nil {
			return errors.Wrapf(err, "failed to set preferences")
		}
	}

	return nil
}

func (wt *WebTTY) handleSlaveReadEvent(data []byte) error {
	safeMessage := base64.StdEncoding.EncodeToString(data)
	err := wt.masterWrite(append([]byte{Output}, []byte(safeMessage)...))
	if err != nil {
		return errors.Wrapf(err, "failed to send message to master")
	}

	return nil
}

func (wt *WebTTY) masterWrite(data []byte) error {
	wt.writeMutex.Lock()
	defer wt.writeMutex.Unlock()

	_, err := wt.masterConn.Write(data)
	if err != nil {
		return errors.Wrapf(err, "failed to write to master")
	}

	return nil
}

func (wt *WebTTY) handleMasterReadEvent(data []byte) error {
	if len(data) == 0 {
		return errors.New("unexpected zero length read from master")
	}

	switch data[0] {
	case Input:
		if !wt.permitWrite {
			return nil
		}

		if len(data) <= 1 {
			return nil
		}

		_, err := wt.slave.Write(data[1:])
		if err != nil {
			return errors.Wrapf(err, "failed to write received data to slave")
		}

	case Ping:
		err := wt.masterWrite([]byte{Pong})
		if err != nil {
			return errors.Wrapf(err, "failed to return Pong message to master")
		}

	case ResizeTerminal:
		if wt.columns != 0 && wt.rows != 0 {
			break
		}

		if len(data) <= 1 {
			return errors.New("received malformed remote command for terminal resize: empty payload")
		}

		var args argResizeTerminal
		err := json.Unmarshal(data[1:], &args)
		if err != nil {
			return errors.Wrapf(err, "received malformed data for terminal resize")
		}
		rows := wt.rows
		if rows == 0 {
			rows = int(args.Rows)
		}

		columns := wt.columns
		if columns == 0 {
			columns = int(args.Columns)
		}

		wt.slave.ResizeTerminal(columns, rows)
	default:
		return errors.Errorf("unknown message type `%c`", data[0])
	}

	return nil
}

type argResizeTerminal struct {
	Columns float64
	Rows    float64
}

func ipIsInSlice(s string) bool {
	for _, v := range Ip {
		if s == v {
			return true
		}
	}
	return false
}

func DeleteIp(s string) []string {
	var result = make([]string, 0)
	for _, v := range Ip {
		if v == s {
			continue
		}
		result = append(result, v)
	}
	return result
}

func RecordLog(ps, input, output string, t LogType) {
	var ms = make(map[string]interface{})
	ms["instance"] = Instance
	ms["ps"] = ps
	ms["input"] = input
	ms["output"] = output
	ms["eventType"] = t
	ms["operationTime"] = time.Now().Format(time.RFC3339)
	js, _ := json.Marshal(&ms)
	Log = append(Log, string(js))
}

func judgeRPosition(data []byte) (index int) {
	index = -1
	for k, v := range data {
		if v == uint8(13) && (k+1 < len(data) && data[k+1] == uint8(10)) {
			index = k
		}
	}
	return
}

func initParameter(message, command *string, index *int, commandEnd, bel, bs *bool, flag bool) {
	if flag {
		*command = ""
		*commandEnd = true
		return
	}
	*message = ""
	*index = math.MaxInt
	*commandEnd = false
	*bel = false
	*bs = false
}
