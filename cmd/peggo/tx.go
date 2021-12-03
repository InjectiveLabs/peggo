package peggo

import (
	"github.com/spf13/cobra"
)

func getTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx",
		Short: "Transactions for Peggy (Gravity Bridge) governance and maintenance on the Cosmos chain",
	}

	return cmd
}
