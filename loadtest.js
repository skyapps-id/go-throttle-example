import http from 'k6/http';
import { sleep } from 'k6';
import { Trend, Counter } from 'k6/metrics';
import exec from 'k6/execution';

const BASE_URL = __ENV.URL || 'http://localhost:8080/no-throttle';

const okTime = new Trend('ok_response_time');
const busyCount = new Counter('busy_count');
const timeoutCount = new Counter('timeout_count');
const okCount = new Counter('ok_count');

export const options = {
  scenarios: {
    ramping: {
      executor: 'ramping-vus',
      startVUs: 10,
      stages: [
        { duration: '5s', target: 30 },
        { duration: '5s', target: 50 },
        { duration: '5s', target: 70 },
        { duration: '20s', target: 100 },
        { duration: '5s', target: 50 },
      ],
    },
  },
};

export function handleSummary(data) {
  const ok = data.metrics.ok_count ? data.metrics.ok_count.values.count : 0;
  const busy = data.metrics.busy_count ? data.metrics.busy_count.values.count : 0;
  const timeout = data.metrics.timeout_count ? data.metrics.timeout_count.values.count : 0;

  let output = '========== LOAD TEST RESULTS ==========\n\n';

  output += '--- Summary ---\n';
  output += `Total requests:  ${data.metrics.http_reqs.values.count}\n`;
  output += `200 OK:          ${ok}\n`;
  output += `503 Server Busy: ${busy}\n`;
  output += `408 Timeout:     ${timeout}\n\n`;

  const okRt = data.metrics.ok_response_time ? data.metrics.ok_response_time.values : null;

  if (okRt && okRt.med !== null && okRt.med !== undefined) {
    output += '--- 200 OK Response Times ---\n';
    output += `  min:  ${okRt.min.toFixed(2)}ms\n`;
    output += `  avg:  ${okRt.avg.toFixed(2)}ms\n`;
    output += `  med:  ${okRt.med.toFixed(2)}ms\n`;
    output += `  p90:  ${okRt['p(90)'].toFixed(2)}ms\n`;
    output += `  p95:  ${okRt['p(95)'].toFixed(2)}ms\n`;
    output += `  max:  ${okRt.max.toFixed(2)}ms\n\n`;
  } else {
    output += '--- 200 OK Response Times ---\n  (no successful requests)\n\n';
  }

  output += '--- HTTP Duration (all) ---\n';
  const hd = data.metrics.http_req_duration.values;
  output += `  min:  ${hd.min.toFixed(2)}ms\n`;
  output += `  avg:  ${hd.avg.toFixed(2)}ms\n`;
  output += `  med:  ${hd.med.toFixed(2)}ms\n`;
  output += `  max:  ${hd.max.toFixed(2)}ms\n`;

  return {
    'stdout': output,
  };
}

let okNum = 0;

export default function () {
  const res = http.get(BASE_URL);

  if (res.status === 200) {
    okNum++;
    okCount.add(1);
    okTime.add(res.timings.duration);
    console.log(`[${okNum.toString().padStart(3)}] 200 OK - ${res.timings.duration.toFixed(0)}ms`);
  } else if (res.status === 503) {
    busyCount.add(1);
    console.log(`[---] 503 BUSY`);
  } else if (res.status === 408) {
    timeoutCount.add(1);
    console.log(`[---] 408 TIMEOUT`);
  } else {
    console.log(`[---] ${res.status} - ${res.body}`);
    exec.test.abort();
  }
}
