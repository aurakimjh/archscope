import jdk.jfr.Recording;
import jdk.jfr.Event;
import jdk.jfr.Name;
import java.nio.file.Path;
class SyntheticRecording { public static void main(String[] args) throws Exception {
 try (Recording recording = new Recording()) {
  recording.enable("synthetic.fixture"); recording.start();
  new SyntheticEvent().commit(); recording.stop(); recording.dump(Path.of(args[0]));
 }
}}
@Name("synthetic.fixture") class SyntheticEvent extends Event { public int value = 0; }
