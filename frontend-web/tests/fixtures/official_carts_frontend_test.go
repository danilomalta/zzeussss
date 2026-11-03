package localapi
import("context";"encoding/json";"net";"os";"os/exec";"testing";"time")
func TestOfficialCartsFrontendBridgeIsolated(t *testing.T){
 f:=salesSetup(t);listener,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)};go func(){_=f.app.Listener(listener)}();defer f.app.Shutdown();defer listener.Close()
 session,_:=json.Marshal(map[string]any{"tenant_id":f.owner.TenantID,"store_id":f.owner.StoreID,"identity_id":f.owner.OwnerID,"device_id":f.device.DeviceID,"expires_unix":time.Now().Add(time.Minute).Unix()});items,_:=json.Marshal(salesInput(f).Items)
 ctx,cancel:=context.WithTimeout(t.Context(),45*time.Second);defer cancel();cmd:=exec.CommandContext(ctx,"node",os.Getenv("TITAN_OFFICIAL_CARTS_SCRIPT"));cmd.Env=append(os.Environ(),"TITAN_BRIDGE_URL=http://"+listener.Addr().String(),"TITAN_BRIDGE_TOKEN="+f.token,"TITAN_BRIDGE_SESSION="+string(session),"TITAN_POS_ITEMS="+string(items));output,e:=cmd.CombinedOutput();if e!=nil{t.Fatalf("frontend session contract: %v\n%s",e,output)};t.Log(string(output))
 var count int;if e=f.db.QueryRow(`SELECT count(*) FROM sales`).Scan(&count);e!=nil||count!=1{t.Fatal(count,e)}
 assertStockBalance(t,f,f.shelf,5000)
}
