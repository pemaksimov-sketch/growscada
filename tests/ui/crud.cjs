const assert = require('node:assert/strict');

// Run against the server and browser owned by smoke.cjs. All creations and
// deletions use actual UI controls; REST only observes persisted results.
module.exports = async function checkCRUD(page, api, until, output) {
  const clickMode = name => page.getByText(name, { exact: true }).click();
  const acceptDialog = dialog => dialog.accept();
  page.on('dialog', acceptDialog);
  await clickMode('Library');
  await page.getByRole('button', { name: 'Create', exact: true }).click();
  const newType = page.getByText('New Widget Type', { exact: true });
  await newType.waitFor();
  await newType.dblclick();
  await page.locator('#widget-type-name').fill('CRUD browser type');
  await page.locator('#widget-type-name').press('Enter');
  await page.getByText('CRUD browser type', { exact: true }).waitFor();
  let type;
  await until(async () => {
    type = (await api('/widget-types')).widget_types.find(t => t.name === 'CRUD browser type');
    return !!type;
  });
  const html = '<div id="crud-preview">CRUD saved template</div>';
  await page.locator('#html-template').fill(html);
  await page.getByRole('button', { name: 'Apply', exact: true }).first().click();
  await until(async () => (await api(`/widget-types/${type.id}`)).html_template === html);
  const release = await api('/libraries/00000000-0000-4000-8000-000000000003/releases', 'POST');
  await api('/projects/00000000-0000-4000-8000-000000000001/libraries', 'POST', {
    library_id: '00000000-0000-4000-8000-000000000003', release_id: release.id,
  });

  await clickMode('Project');
  // A previous Project instance can remain visible during the mode switch.
  // The new type was created in Library, so its thumbnail proves that the
  // mounted Project instance has finished its initial API load.
  await page.locator(`[data-widget-type-id="${type.id}"]`).waitFor();
  const createdScene = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/scenes'));
  await page.locator('#create-scene').click();
  assert.equal((await createdScene).status(), 201);
  await until(async () => await page.locator('#scene-name').inputValue() === 'New Scene');
  await page.locator('#scene-name').fill('CRUD browser scene');
  await page.locator('#scene-name').press('Tab');
  let scene;
  await until(async () => {
    scene = (await api('/scenes')).scenes.find(s => s.name === 'CRUD browser scene');
    return !!scene && Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) === scene.version;
  });
  await page.locator('#scene-background').fill('<div>CRUD scene background</div>');
  await page.locator('#scene-background').press('Tab');
  await until(async () => {
    const saved = await api(`/scenes/${scene.id}`);
    return saved.background_html.includes('CRUD scene background') &&
      Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) === saved.version;
  });
  // Deliberately overlap two real edits while the first HTTP write is slow.
  // They share the Scene version and must both persist without a self-conflict.
  const beforeRapid = await api(`/scenes/${scene.id}`);
  const sceneURL = `**/scenes/${scene.id}`;
  let delayed = false;
  const delayFirstWrite = async route => {
    if (route.request().method() === 'PUT' && !delayed) {
      delayed = true;
      await new Promise(resolve => setTimeout(resolve, 700));
    }
    await route.continue();
  };
  await page.route(sceneURL, delayFirstWrite);
  await page.locator('#scene-name').fill('CRUD browser scene renamed');
  await page.locator('#scene-name').press('Tab');
  await page.locator('#scene-background').fill('<div>CRUD scene background rapid</div>');
  await page.locator('#scene-background').press('Tab');
  await until(async () => {
    const saved = await api(`/scenes/${scene.id}`);
    return saved.name === 'CRUD browser scene renamed' && saved.background_html.includes('background rapid') &&
      saved.version === beforeRapid.version + 2 &&
      Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) === saved.version;
  });
  assert.ok(delayed, 'slow-save regression did not exercise a PUT');
  await page.unroute(sceneURL, delayFirstWrite);
  await page.locator(`[data-widget-type-id="${type.id}"]`).dragTo(page.locator('#scene-canvas'), {
    targetPosition: { x: 200, y: 180 },
  });
  let widget;
  await until(async () => {
    widget = (await api(`/scenes/${scene.id}/widgets`)).widgets[0];
    return !!widget;
  });
  await page.locator('#widget-name').waitFor();
  await page.locator('#scene-canvas').click({ position: { x: 600, y: 400 } });
  await page.locator('#scene-name').waitFor();
  const widgetBox = await page.locator(`#w-preview-${widget.id}`).boundingBox();
  assert.ok(widgetBox);
  await page.mouse.click(widgetBox.x + 20, widgetBox.y + 20);
  await page.locator('#widget-name').waitFor();
  await page.locator('#widget-name').fill('CRUD browser widget');
  await page.locator('#widget-name').press('Tab');
  await until(async () => (await api(`/scenes/${scene.id}/widgets/${widget.id}`)).name === 'CRUD browser widget');
  let savedWidget = await api(`/scenes/${scene.id}/widgets/${widget.id}`);
  await until(async () => Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) === savedWidget.scene_version);

  // Alignment feedback is visible during the gesture and does not snap.
  const movingBox = await page.locator(`#w-preview-${widget.id}`).boundingBox();
  assert.ok(movingBox);
  await page.mouse.move(movingBox.x + 20, movingBox.y + 20);
  await page.mouse.down();
  await page.mouse.move(movingBox.x + 20 + 2 - savedWidget.position.x, movingBox.y + 20, { steps: 12 });
  const guide = page.locator('.scene-guide[data-axis="x"][data-position="0.00"]');
  await guide.waitFor();
  assert.equal(await guide.evaluate(el => getComputedStyle(el).pointerEvents), 'none');
  await page.screenshot({ path: require('node:path').join(output, 'guides.png') });
  await page.mouse.up();
  await until(async () => (await api(`/scenes/${scene.id}/widgets/${widget.id}`)).position.x === 2);
  await until(async () => await page.locator('.scene-guide').count() === 0);
  savedWidget = await api(`/scenes/${scene.id}/widgets/${widget.id}`);
  await until(async () => Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) === savedWidget.scene_version);

  // Background and grid must leave resize and rotation handles usable too.
  const resize = await page.getByLabel('Resize widget', { exact: true }).boundingBox();
  assert.ok(resize);
  await page.mouse.move(resize.x + resize.width / 2, resize.y + resize.height / 2);
  await page.mouse.down();
  await page.mouse.move(resize.x + resize.width / 2 + 30, resize.y + resize.height / 2 + 20, { steps: 8 });
  await page.mouse.up();
  await until(async () => {
    const resized = await api(`/scenes/${scene.id}/widgets/${widget.id}`);
    return resized.size.width === savedWidget.size.width + 30 && resized.size.height === savedWidget.size.height + 20;
  });
  savedWidget = await api(`/scenes/${scene.id}/widgets/${widget.id}`);
  await until(async () => Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) === savedWidget.scene_version);
  const rotate = await page.getByLabel('Rotate widget', { exact: true }).boundingBox();
  assert.ok(rotate);
  await page.mouse.move(rotate.x + rotate.width / 2, rotate.y + rotate.height / 2);
  await page.mouse.down();
  await page.mouse.move(rotate.x + rotate.width / 2 + 40, rotate.y + rotate.height / 2 + 40, { steps: 8 });
  await page.mouse.up();
  await until(async () => Math.abs((await api(`/scenes/${scene.id}/widgets/${widget.id}`)).rotation.degrees) > 80);
  savedWidget = await api(`/scenes/${scene.id}/widgets/${widget.id}`);
  await until(async () => Number(await page.locator('#scene-canvas').getAttribute('data-scene-version')) === savedWidget.scene_version);

  await clickMode('Project');
  await page.getByRole('button', { name: 'Delete Widget', exact: true }).click();
  await until(async () => (await api(`/scenes/${scene.id}/widgets`)).widgets.length === 0);
  await page.getByLabel('Delete scene CRUD browser scene renamed', { exact: true }).click();
  await until(async () => !(await api('/scenes')).scenes.some(s => s.id === scene.id));
  await clickMode('Library');
  await page.getByText('CRUD browser type', { exact: true }).click();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await until(async () => !(await api('/widget-types')).widget_types.some(t => t.id === type.id));

  await clickMode('Project');
  await page.getByText('Tags', { exact: true }).first().click();
  await page.getByRole('button', { name: 'Create', exact: true }).click();
  await page.locator('#new-tag-name').fill('qa.crud');
  await page.locator('#new-tag-type').selectOption('integer');
  await page.getByRole('button', { name: 'Create', exact: true }).last().click();
  await page.getByText('Tag Properties', { exact: true }).waitFor();
  let tag;
  await until(async () => {
    tag = (await api('/tags')).tags.find(t => t.name === 'qa.crud');
    return !!tag && tag.type === 'integer';
  });
  // Tag metadata is read-only in Project; values are written by devices.
  await api(`/tags/${tag.id}/value`, 'PATCH', { value: '12345', quality: 'good', version: tag.version });
  await page.getByText('12345', { exact: true }).waitFor();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await until(async () => !(await api('/tags')).tags.some(t => t.id === tag.id));
  await page.getByText('Scenes', { exact: true }).click();
  await page.getByText('Browser QA', { exact: true }).click();
  console.log('PASS: UI CRUD, released type use, background drop/resize/rotation');
  page.off('dialog', acceptDialog);
};
