import jdk.jfr.Recording;
import jdk.jfr.Event;
import jdk.jfr.Name;
import java.nio.file.Path;
class LargeRecording {
 static final String PADDING = "sanitized-".repeat(32);
 static void emit(int sequence,int depth) {
  if(depth>0){emit(sequence,depth-1);return;}
  SyntheticEvent event=new SyntheticEvent();event.sequence=sequence;event.payload=PADDING+sequence;event.commit();
 }
 public static void main(String[] args) throws Exception {
  try(Recording recording=new Recording()){
   recording.enable("synthetic.r10.large").withStackTrace();recording.start();
   for(int i=0;i<30000;i++)emit(i,3);
   recording.stop();recording.dump(Path.of(args[0]));
  }
 }
}
@Name("synthetic.r10.large") class SyntheticEvent extends Event { public int sequence; public String payload; }
