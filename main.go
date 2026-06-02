// openerp — 金蝶云·星空 (Kingdee K3 Cloud) ERP CLI, built for humans and AI agents.
package main

import (
	"os"

	"github.com/zhoujw/openerp-cli/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
