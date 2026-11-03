package localapi
import("context";"encoding/json";"net";"os";"os/exec";"testing";"time")
func TestCashVaultsFrontendBridgeIsolated(t *testing.T){
 f,receiver:=vaultSetup(t);before:=vaultCounts(t,f)
 listener,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)};go func(){_=f.app.Listener(listener)}();defer f.app.Shutdown();defer listener.Close()
 session,_:=json.Marshal(map[string]any{"tenant_id":f.owner.TenantID,"store_id":f.owner.StoreID,"identity_id":f.owner.OwnerID,"device_id":f.device.DeviceID,"expires_unix":time.Now().Add(time.Minute).Unix()})
 ctx,cancel:=context.WithTimeout(t.Context(),45*time.Second);defer cancel();cmd:=exec.CommandContext(ctx,"node",os.Getenv("TITAN_VAULTS_SCRIPT"));cmd.Env=append(os.Environ(),"TITAN_BRIDGE_URL=http://"+listener.Addr().String(),"TITAN_BRIDGE_TOKEN="+f.token,"TITAN_BRIDGE_RECEIVER="+receiver,"TITAN_BRIDGE_SESSION="+string(session));output,e:=cmd.CombinedOutput();if e!=nil{t.Fatalf("frontend vault contract: %v\n%s",e,output)};t.Log(string(output))
 after:=vaultCounts(t,f);if after[0]!=before[0]+1||after[1]!=before[1]+1||after[2]!=before[2]{t.Fatal("money/stock duplicated or count adjusted balance",before,after)}
 if a,b:=readVault(t,f,f.token,"vault-one",200),readVault(t,f,f.token,"vault-two",200);a.State!="archived"||a.BalanceCents!=0||b.BalanceCents!=240{t.Fatal(a,b)}
}
