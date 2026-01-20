package ports

import "strconv"

// L7ProtocolFromDPort returns a short L7 protocol name derived from destination port.
//
// The project requirement:
// - if port is standard => meaningful name (e.g. 443=https)
// - otherwise => "unknown"
// - if protocol doesn't have ports => caller should pass "0" and we return "na"
func L7ProtocolFromDPort(dport string) string {
	if dport == "" {
		return "unknown"
	}
	if dport == "0" {
		return "na"
	}

	p, err := strconv.Atoi(dport)
	if err != nil {
		return "unknown"
	}

	// Keep this list intentionally small and conservative.
	switch p {
	case 20, 21:
		return "ftp"
	case 22:
		return "ssh"
	case 23:
		return "telnet"
	case 25:
		return "smtp"
	case 53:
		return "dns"
	case 80:
		return "http"
	case 110:
		return "pop3"
	case 143:
		return "imap"
	case 389:
		return "ldap"
	case 443:
		return "https"
	case 465, 587:
		return "smtp"
	case 631:
		return "ipp"
	case 993:
		return "imaps"
	case 995:
		return "pop3s"
	case 3306:
		return "mysql"
	case 5432:
		return "postgres"
	case 6379:
		return "redis"
	case 9200:
		return "elasticsearch"
	}

	return "unknown"
}

