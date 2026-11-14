public class NativeRecordingFixture {
  public static void main(String[] args) throws Exception {
    long until = System.nanoTime() + 4_000_000_000L;
    double value = 0;
    while (System.nanoTime() < until) {
      for (int i = 1; i < 100000; i++) value += Math.sqrt(i);
      Thread.sleep(2);
    }
    System.out.println("owned synthetic recording complete: " + Double.isFinite(value));
  }
}
