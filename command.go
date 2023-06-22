package main

type Command int

const (
	CmdUnknown Command = iota
	CmdTunnel
	CmdTTL
)

func (c Command) String() string {
	switch c {
	case CmdTunnel:
		return "tunnel"
	case CmdTTL:
		return "ttl"
	}
	return "unknown"
}

func ParseCmd(cmd string) Command {
	switch cmd {
	case "tunnel":
		return CmdTunnel
	case "ttl":
		return CmdTTL
	}
	return CmdUnknown
}
