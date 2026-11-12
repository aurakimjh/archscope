import json,socket,subprocess,time,platform

def snapshot():
 start=time.time_ns();raw=subprocess.check_output(['ss','-s'],text=True);end=time.time_ns()
 return {'capture_start_ns':str(start),'capture_end_ns':str(end),'summary':raw}
rows=[snapshot()];listener=socket.socket();listener.bind(('127.0.0.1',0));listener.listen(8);pairs=[]
for i in range(4):
 client=socket.socket();client.connect(listener.getsockname());server,_=listener.accept();pairs.append((client,server))
rows.append(snapshot())
for a,b in pairs:a.close();b.close()
listener.close();rows.append(snapshot())
c={'contract':'linux-ss-summary.v1','collector_version':'1.0.0','iproute2_version':'6.1.0','kernel_release':platform.release(),'host_id':'fixture-linux-host','boot_id':'fixture-linux-boot','network_namespace':'fixture-isolated-netns','snapshots':rows}
print(json.dumps(c,indent=2))
