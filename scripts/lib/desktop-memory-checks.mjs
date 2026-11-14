const median = values => {
  const sorted = [...values].sort((a,b) => a-b), mid = Math.floor(sorted.length/2);
  return sorted.length % 2 ? sorted[mid] : (sorted[mid-1]+sorted[mid])/2;
};

// Screening budgets are fixed before a run, not universal leak-free criteria.
export const memoryPlateauPolicy = Object.freeze({
  measuredCycles: 8, window: 3,
  treePrivateBytes: { delta: 64*1048576, perCycle: 4*1048576 },
  rootPrivateBytes: { delta: 16*1048576, perCycle: 1048576 },
  rootHandles: { delta: 24, perCycle: 2 },
  rootThreads: { delta: 4, perCycle: 0.5 },
});

export function assessMemoryPlateau(points) {
  if (points.length !== memoryPlateauPolicy.measuredCycles) throw Error('eight matched post-warmup idle samples required');
  const metrics = {};
  for (const key of ['treePrivateBytes','rootPrivateBytes','rootHandles','rootThreads']) {
    const values = points.map(p => p[key]);
    if (values.some(v => !Number.isSafeInteger(v) || v < 0)) throw Error('invalid process metric');
    const slopes = [];
    for (let i=0;i<values.length;i++) for (let j=i+1;j<values.length;j++) slopes.push((values[j]-values[i])/(j-i));
    const firstMedian = median(values.slice(0,3)), lastMedian = median(values.slice(-3));
    const slopePerCycle = median(slopes), delta = lastMedian-firstMedian;
    const budget = memoryPlateauPolicy[key];
    metrics[key] = { firstMedian,lastMedian,delta,slopePerCycle,budget,withinBudget:delta<=budget.delta && slopePerCycle<=budget.perCycle };
  }
  const withinBudget = Object.values(metrics).every(m => m.withinBudget);
  return { policy:memoryPlateauPolicy,metrics,withinBudget,classification:withinBudget?'BOUNDED_IN_THIS_RUN':'GROWTH_REQUIRES_DIAGNOSIS' };
}
