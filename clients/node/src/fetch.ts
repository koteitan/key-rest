import { connect } from 'node:net';
import { homedir } from 'node:os';
import { join } from 'node:path';

interface DaemonRequest {
  type: 'http';
  method: string;
  url: string;
  headers: Record<string, string>;
  body: string | null;
}

interface DaemonResponse {
  status?: number;
  statusText?: string;
  headers?: Record<string, string>;
  body?: string;
  error?: { code: string; message: string };
}

interface CreateFetchOptions {
  socketPath?: string;
}

/**
 * Creates a fetch-compatible function that routes requests through key-rest-daemon.
 */
export function createFetch(options?: CreateFetchOptions) {
  const socketPath = options?.socketPath ?? join(homedir(), '.key-rest', 'key-rest.sock');

  return async function keyRestFetch(
    input: string | URL,
    init?: RequestInit,
  ): Promise<Response> {
    const url = typeof input === 'string' ? input : input.toString();
    const method = init?.method ?? 'GET';

    // Extract headers
    const headers: Record<string, string> = {};
    if (init?.headers) {
      if (init.headers instanceof Headers) {
        init.headers.forEach((value, key) => { headers[key] = value; });
      } else if (Array.isArray(init.headers)) {
        for (const [key, value] of init.headers) {
          headers[key] = value;
        }
      } else {
        Object.assign(headers, init.headers);
      }
    }

    // Extract body
    let body: string | null = null;
    if (init?.body != null) {
      if (typeof init.body === 'string') {
        body = init.body;
      } else {
        body = JSON.stringify(init.body);
      }
    }

    const request: DaemonRequest = { type: 'http', method, url, headers, body };
    const response = await sendToDaemon(socketPath, request);

    if (response.error) {
      throw new Error(`[${response.error.code}] ${response.error.message}`);
    }

    const responseHeaders = new Headers(response.headers ?? {});
    return new Response(response.body ?? '', {
      status: response.status ?? 200,
      statusText: response.statusText?.replace(/^\d+\s*/, '') ?? 'OK',
      headers: responseHeaders,
    });
  };
}

function sendToDaemon(socketPath: string, request: DaemonRequest): Promise<DaemonResponse> {
  return new Promise((resolve, reject) => {
    const socket = connect(socketPath, () => {
      socket.write(JSON.stringify(request) + '\n');
    });

    let data = '';
    socket.on('data', (chunk) => {
      data += chunk.toString();
      const newlineIdx = data.indexOf('\n');
      if (newlineIdx !== -1) {
        const line = data.slice(0, newlineIdx);
        socket.end();
        try {
          resolve(JSON.parse(line));
        } catch (e) {
          reject(new Error('failed to parse daemon response'));
        }
      }
    });

    socket.on('error', (err) => {
      reject(new Error(`failed to connect to key-rest-daemon: ${err.message}`));
    });

    socket.on('end', () => {
      if (data && !data.includes('\n')) {
        try {
          resolve(JSON.parse(data));
        } catch {
          reject(new Error('incomplete response from daemon'));
        }
      }
    });
  });
}
