const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright');

const root = path.resolve(__dirname, '../..');
const output = path.join(root, 'artifacts/ui');
fs.mkdirSync(output, { recursive: true });
const log = fs.openSync(path.join(output, 'server.log'), 'w');
function startServer() { return spawn('./growscada_combined_server', [], {
  cwd: path.join(root, 'bin/growscada_combined_server'), stdio: ['ignore', log, log],
}); }
let server = startServer();
const exited = child => new Promise(resolve => child.once('exit', (code, signal) => resolve({ code, signal })));
let serverExit = exited(server);
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(check, timeout = 30000) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    try { if (await check()) return; } catch (error) { last = error; }
    await sleep(100);
  }
  throw last || new Error('Condition timed out');
}
async function api(resource, method = 'GET', body) {
  const response = await fetch(`http://localhost:9090/api/v0${resource}`, {
    method, headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  assert.ok(response.ok, `${method} ${resource}: ${response.status} ${await response.clone().text()}`);
  return response.json();
}

(async () => {
  let browser;
  let page;
  const responses = [];
  try {
    await until(async () => (await fetch('http://localhost:9090/api/v0/scenes')).ok);
    const scene = await api('/scenes', 'POST', {
      name: 'Browser QA', width: 900, height: 600,
      background_html: '<div style="background:#1a1a2e;width:900px;height:600px">QA background</div>',
    });
    const tag = await api('/tags', 'POST', { name: 'qa.counter', type: 'integer', value: '9007199254740993', quality: 'good' });
    const createdType = await api('/widget-types', 'POST', {
      name: 'QA exact counter', html_template: '<div id="exact" style="color:white"></div>',
      script: "function render(inputs){document.querySelector('#exact').textContent=BigInt(inputs.n.raw).toString();}",
      script_language: 'javascript', default_width: 350, default_height: 80,
      input_ports: [{ name: 'n', description: 'Counter', type_hint: 'integer' }],
    });
    const type = await api(`/widget-types/${createdType.id}`);
    const widget = await api(`/scenes/${scene.id}/widgets`, 'POST', {
      name: 'QA counter', position: { x: 100, y: 100, z: 0 }, size: { width: 350, height: 80 },
      origin: { x: 0, y: 0 }, rotation: { degrees: 0 }, type_id: type.id,
      labels: [], port_bindings: [{ port_name: 'n', tag_id: tag.id }], scene_version: 1,
    });
    browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
    await context.addInitScript(({ sceneID }) => {
      // Playwright also injects this into sandboxed srcdoc frames. Only the
      // application window owns preferences; sandbox frames cannot use storage.
      if (window !== window.top) return;
      localStorage.setItem('root:mode', JSON.stringify('project'));
      localStorage.setItem('project:sceneID', JSON.stringify(sceneID));
      localStorage.setItem('operation:sceneID', JSON.stringify(sceneID));
    }, { sceneID: scene.id });
    page = await context.newPage();
    page.on('response', response => {
      if (response.url().includes('/api/v0/') && !response.url().endsWith('/events')) {
        responses.push({ method: response.request().method(), url: response.url(), status: response.status() });
      }
    });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto('http://localhost:8080');
    const background = page.locator('iframe[title="Scene background"]');
    await background.waitFor({ state: 'visible', timeout: 60000 });
    assert.equal(await background.getAttribute('sandbox'), '');
    assert.equal(await background.evaluate(el => getComputedStyle(el).pointerEvents), 'none');
    assert.equal(await background.evaluate(el => getComputedStyle(el.nextElementSibling).pointerEvents), 'none');
    assert.match(await background.getAttribute('srcdoc'), /#1a1a2e/);
    const initialBox = await page.locator(`#w-preview-${widget.id}`).boundingBox();
    assert.ok(initialBox);
    await page.mouse.click(initialBox.x + 30, initialBox.y + 30);
    await page.locator(`#sim-${widget.id}-n`).fill('9007199254740993');
    const preview = page.frameLocator(`#w-preview-${widget.id}`).locator('#exact');
    await until(async () => await preview.textContent() === '9007199254740993');
    await page.screenshot({ path: path.join(output, 'project.png') });

    // A different scene's widget must not reload the selected scene or the
    // complete scene list. Listen before making the change through REST.
    const other = await api('/scenes', 'POST', { name: 'Other scene', width: 300, height: 200, background_html: '' });
    await sleep(1000); // allow the scene-created refresh to settle
    const requests = [];
    const recordRequest = req => { if (req.method() === 'GET') requests.push(new URL(req.url()).pathname); };
    page.on('request', recordRequest);
    await api(`/tags/${tag.id}/value`, 'PATCH', {value:'9007199254740993', quality:'good', version:1});
    await sleep(500);
    assert.ok(!requests.includes('/api/v0/tags'), 'Project reread all tags on a value event: '+JSON.stringify(requests));
    const otherWidget = await api(`/scenes/${other.id}/widgets`, 'POST', {
      name: 'Other counter', position: { x: 10, y: 10, z: 0 }, size: { width: 150, height: 80 },
      origin: { x: 0, y: 0 }, rotation: { degrees: 0 }, type_id: type.id, labels: [], port_bindings: [], scene_version: 1,
    });
    await until(() => requests.includes(`/api/v0/scenes/${other.id}`));
    await sleep(300);
    assert.ok(!requests.includes('/api/v0/scenes'), JSON.stringify(requests));
    assert.ok(!requests.includes(`/api/v0/scenes/${scene.id}/widgets`), JSON.stringify(requests));
    requests.length = 0;
    await api(`/scenes/${other.id}/widgets/${otherWidget.id}`, 'DELETE');
    await until(() => requests.includes(`/api/v0/scenes/${other.id}`));
    assert.ok(!requests.includes('/api/v0/scenes'), JSON.stringify(requests));
    page.off('request', recordRequest);

    // The background and grid must not consume widget dragging.
    const box = await page.locator(`#w-preview-${widget.id}`).boundingBox();
    assert.ok(box);
    await page.mouse.move(box.x + 30, box.y + 30);
    await page.mouse.down();
    await page.mouse.move(box.x + 70, box.y + 60, { steps: 8 });
    await page.mouse.up();
    await until(async () => {
      const moved = await api(`/scenes/${scene.id}/widgets/${widget.id}`);
      return moved.position.x === 140 && moved.position.y === 130;
    });

    // A change made by another client arrives through SSE without reloading.
    await page.getByText('Browser QA', { exact: true }).click();
    const sceneName = page.locator('#scene-name');
    await sceneName.waitFor({ state: 'visible' });
    await sceneName.fill('Unsaved scene name');
    const current = await api(`/scenes/${scene.id}`);
    await api(`/scenes/${scene.id}`, 'PUT', {
      name: current.name, width: current.width, height: current.height, version: current.version,
      background_html: '<div style="background:#16213e;width:900px;height:600px">Updated QA background</div>',
    });
    await until(async () => (await background.getAttribute('srcdoc')).includes('Updated QA background'));
    assert.equal(await sceneName.inputValue(), 'Unsaved scene name', 'SSE erased an unsaved Project edit');
    await sceneName.fill(current.name);
    const beforeNameSave = await api(`/scenes/${scene.id}`);
    await sceneName.press('Tab');
    await until(async () => Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) > beforeNameSave.version);
    const backgroundEditor = page.locator('#scene-background');
    await backgroundEditor.fill('<div style="background:#16213e;width:900px;height:600px">Applied QA background</div>');
    await backgroundEditor.press('Tab');
    await until(async () => (await background.getAttribute('srcdoc')).includes('Applied QA background'));
    await page.getByText('Operation', { exact: true }).click();
    await background.waitFor({ state: 'visible' });
    await until(async () => {
      for (const frame of page.frames()) {
        if (await frame.locator('#exact').count() && await frame.locator('#exact').textContent() === '9007199254740993') return true;
      }
      return false;
    });
    await page.screenshot({ path: path.join(output, 'operation.png') });

    await page.getByText('Library', { exact: true }).click();
    await page.getByText('QA exact counter', { exact: true }).click();
    const libraryInput = page.locator('#input-data-n');
    const libraryValue = page.frameLocator('#preview-iframe').locator('#exact');
    for (const value of ['9007199254740991', '9007199254740993', '-9007199254740993', '9223372036854775807', '-9223372036854775808']) {
      await libraryInput.fill(value);
      await until(async () => await libraryValue.textContent() === value);
    }
    const draftHTML = type.html_template + '<span id="unsaved">Local draft</span>';
    await page.locator('#html-template').fill(draftHTML);
    const changedType = await api(`/widget-types/${type.id}`, 'PUT', {
      name: 'QA remote rename', html_template: type.html_template, script: type.script,
      script_language: type.script_language, default_width: type.default_width,
      default_height: type.default_height, input_ports: type.input_ports, version: type.version,
    });
    await page.getByText('QA remote rename', { exact: true }).waitFor();
    assert.equal(await page.locator('#html-template').inputValue(), draftHTML, 'SSE erased an unsaved Library edit');
    assert.equal((await api(`/widget-types/${type.id}`)).html_template, type.html_template);
    await page.screenshot({ path: path.join(output, 'library.png') });

    // Remove a port through the actual Library editor. The policy must
    // reconcile the saved binding before a subsequent Project drag.
    await page.locator('#script-editor').fill("function render(inputs){document.querySelector('#exact').textContent=inputs.n?BigInt(inputs.n.raw).toString():'no input';}");
    await page.getByRole('button', { name: '✕', exact: true }).click();
    await page.getByRole('button', { name: 'Apply', exact: true }).first().click();
    await until(async () => (await api(`/scenes/${scene.id}/widgets/${widget.id}`)).port_bindings.length === 0);
    assert.ok((await api(`/widget-types/${type.id}`)).version > changedType.version);
    await page.getByText('Project', { exact: true }).click();
    const reconciledBox = await page.locator(`#w-preview-${widget.id}`).boundingBox();
    assert.ok(reconciledBox);
    const conflicts = [];
    page.on('response', response => { if (response.status() === 409) conflicts.push(response.url()); });
    await page.mouse.move(reconciledBox.x + 30, reconciledBox.y + 30);
    await page.mouse.down();
    await page.mouse.move(reconciledBox.x + 50, reconciledBox.y + 50, { steps: 8 });
    await page.mouse.up();
    await until(async () => (await api(`/scenes/${scene.id}/widgets/${widget.id}`)).position.x === 160);
    assert.deepEqual(conflicts, [], 'Drag after reconciliation conflicted');
    await require('./crud.cjs')(page, api, until);
    assert.deepEqual(errors, [], 'Browser runtime errors');
	await page.getByText('Operation', { exact: true }).click();
	await background.waitFor({ state: 'visible' });

    const stoppedAt = Date.now();
    server.kill('SIGTERM');
    const outcome = await Promise.race([serverExit, sleep(3000).then(() => { throw new Error('Shutdown waited for SSE'); })]);
    assert.equal(outcome.code, 0, JSON.stringify(outcome));
    assert.ok(Date.now() - stoppedAt < 3000);
    await page.getByText(/No connection to the server/).first().waitFor({ timeout: 10000 });
    server = startServer();
    serverExit = exited(server);
    await until(async () => (await fetch('http://localhost:9090/api/v0/scenes')).ok);
    await until(async () => !(await page.getByText(/No connection to the server/).count()), 15000);
    const restartedScene = await api(`/scenes/${scene.id}`);
    await api(`/scenes/${scene.id}`, 'PUT', {
      name: restartedScene.name, width: restartedScene.width, height: restartedScene.height,
      version: restartedScene.version, background_html: '<div>Live after restart</div>',
    });
    await until(async () => (await background.getAttribute('srcdoc')).includes('Live after restart'));
    assert.deepEqual(errors, [], 'Browser runtime errors after reconnect');
    server.kill('SIGTERM');
    assert.equal((await serverExit).code, 0);
    console.log('PASS: background, scoped SSE, draft preservation, exact Integer, reconciliation and restart');
  } catch (error) {
    fs.writeFileSync(path.join(output, 'responses.json'), JSON.stringify(responses, null, 2));
    if (page) {
      fs.writeFileSync(path.join(output, 'page.html'), await page.content().catch(() => ''));
      fs.writeFileSync(path.join(output, 'scenes.json'), JSON.stringify(await api('/scenes').catch(() => ({})), null, 2));
    }
    if (page) await page.screenshot({ path: path.join(output, 'failure.png') }).catch(() => {});
    throw error;
  } finally {
    if (browser) await browser.close();
    if (server.exitCode === null) server.kill('SIGKILL');
    fs.closeSync(log);
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
