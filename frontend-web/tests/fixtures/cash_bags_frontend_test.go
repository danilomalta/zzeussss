package localapi
import("context";"encoding/json";"net";"os";"os/exec";"testing";"time";"reflect")
func TestCashCustodyFrontendBridgeIsolated(t *testing.T){
 f,receiver:=bagSetup(t)
 cashRequest(t,f.httpContractFixture,"close",`{"session_id":"turn-one","operation_id":"close-bags-bridge","declared_cents":700}`,200)
 before:=bagCounts(t,f)
 listener,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{t.Fatal(e)}
 go func(){_=f.app.Listener(listener)}();defer f.app.Shutdown();defer listener.Close()
 session,_:=json.Marshal(map[string]any{"tenant_id":f.owner.TenantID,"store_id":f.owner.StoreID,"identity_id":f.owner.OwnerID,"device_id":f.device.DeviceID,"expires_unix":time.Now().Add(time.Minute).Unix()})
 ctx,cancel:=context.WithTimeout(t.Context(),45*time.Second);defer cancel()
 cmd:=exec.CommandContext(ctx,"node",os.Getenv("TITAN_BAGS_SCRIPT"));cmd.Env=append(os.Environ(),"TITAN_BRIDGE_URL=http://"+listener.Addr().String(),"TITAN_BRIDGE_TOKEN="+f.token,"TITAN_BRIDGE_RECEIVER="+receiver,"TITAN_BRIDGE_SESSION="+string(session))
 output,e:=cmd.CombinedOutput();if e!=nil{t.Fatalf("client contract: %v\n%s",e,output)};t.Log(string(output))
 after:=bagCounts(t,f);if !reflect.DeepEqual(before[:3],after[:3])||after[3]!=1||after[4]!=3{t.Fatal("custody moved money or duplicated",before,after)}
}
