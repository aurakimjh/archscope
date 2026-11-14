// Historical store recovery failure does not invalidate independent platform
// capabilities. Core/capability failures still reject the bootstrap.
export async function loadLiveCaptureBootstrap<T, R>(
  core: PromiseLike<T>,
  recovery: PromiseLike<R[]>,
): Promise<{ core: T; reports: R[]; recoveryError: string | null }> {
  const [loaded, recovered] = await Promise.all([
    core,
    Promise.resolve(recovery).then(
      reports => ({ reports, recoveryError: null }),
      error => ({ reports: [] as R[], recoveryError: error instanceof Error ? error.message : String(error) }),
    ),
  ]);
  return { core: loaded, ...recovered };
}
