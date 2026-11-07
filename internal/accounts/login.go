package accounts

import (
	"context"

	"github.com/azrinaldi/zcode-bridge/internal/auth"
)

// CompleteDeviceFlow waits for approval of a started device flow and stores
// the resulting plan authorization in the account ring. It is shared by the
// CLI `login` command and the dashboard's add-account flow.
func CompleteDeviceFlow(ctx context.Context, flow *auth.DeviceFlow, label string) (Account, error) {
	result, err := auth.PollDeviceFlow(ctx, flow)
	if err != nil {
		return Account{}, err
	}
	claims, _ := auth.DecodeJWT(result.JWT)
	var expiresAt int64
	if e, ok := claims["exp"].(float64); ok {
		expiresAt = int64(e) * 1000
	}
	return Upsert("", label, result.JWT, result.RefreshToken, result.AccessToken, expiresAt), nil
}
