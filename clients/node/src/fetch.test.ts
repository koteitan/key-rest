import { describe, it } from 'node:test';
import * as assert from 'node:assert/strict';
import { createServer } from 'node:net';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir, homedir } from 'node:os';

import { createFetch } from './fetch.js';

interface MockHandler {
  (req: Record<string, unknown>): Record<string, unknown>;
}

function withMockDaemon(handler: MockHandler, fn: (socketPath: string) => Promise<void>): Promise<void> {
  const dir = mkdtempSync(join(tmpdir(), 'key-rest-test-'));
  const socketPath = join(dir, 'test.sock');

  return new Promise<void>((resolve, reject) => {
    const server = createServer((conn) => {
      let data = '';
      conn.on('data', (chunk) => {
        data += chunk.toString();
        const idx = data.indexOf('\n');
        if (idx !== -1) {
          const line = data.slice(0, idx);
          const req = JSON.parse(line);
          const resp = handler(req);
          conn.write(JSON.stringify(resp) + '\n');
          conn.end();
        }
      });
    });

    server.listen(socketPath, async () => {
      try {
        await fn(socketPath);
        resolve();
      } catch (e) {
        reject(e);
      } finally {
        server.close();
        rmSync(dir, { recursive: true, force: true });
      }
    });
  });
}

describe('createFetch', () => {
  it('should send GET request and return response', async () => {
    await withMockDaemon(
      (req) => {
        assert.equal(req.method, 'GET');
        assert.equal(req.url, 'https://api.example.com/data');
        const headers = req.headers as Record<string, string>;
        assert.ok(headers['Authorization']?.includes('key-rest://'));
        return {
          status: 200,
          statusText: '200 OK',
          headers: { 'Content-Type': 'application/json' },
          body: '{"ok":true}',
        };
      },
      async (socketPath) => {
        const fetch = createFetch({ socketPath });
        const resp = await fetch('https://api.example.com/data', {
          headers: { 'Authorization': 'Bearer key-rest://user1/test/key' },
        });
        assert.equal(resp.status, 200);
        const body = await resp.json();
        assert.deepEqual(body, { ok: true });
      },
    );
  });

  it('should send POST request with JSON body', async () => {
    await withMockDaemon(
      (req) => {
        assert.equal(req.method, 'POST');
        assert.equal(req.body, '{"query":"test"}');
        return {
          status: 200,
          body: '{"result":"ok"}',
        };
      },
      async (socketPath) => {
        const fetch = createFetch({ socketPath });
        const resp = await fetch('https://api.example.com/search', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: '{"query":"test"}',
        });
        assert.equal(resp.status, 200);
        const body = await resp.json();
        assert.deepEqual(body, { result: 'ok' });
      },
    );
  });

  it('should throw on error response from daemon', async () => {
    await withMockDaemon(
      () => ({
        error: {
          code: 'KEY_NOT_FOUND',
          message: "key 'user1/test/key' not found",
        },
      }),
      async (socketPath) => {
        const fetch = createFetch({ socketPath });
        await assert.rejects(
          () => fetch('https://api.example.com/', {
            headers: { 'Authorization': 'Bearer key-rest://user1/test/key' },
          }),
          (err: Error) => {
            assert.ok(err.message.includes('KEY_NOT_FOUND'));
            return true;
          },
        );
      },
    );
  });

  it('should throw on connection error', async () => {
    const fetch = createFetch({ socketPath: '/nonexistent/socket.sock' });
    await assert.rejects(
      () => fetch('https://example.com/'),
      (err: Error) => {
        assert.ok(err.message.includes('failed to connect'));
        return true;
      },
    );
  });

  it('should use default socket path', () => {
    // createFetch without options should not throw
    const fetch = createFetch();
    assert.equal(typeof fetch, 'function');
    // We can't easily inspect the socket path, but verify it creates a callable function
  });
});
