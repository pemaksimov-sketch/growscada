const assert = require('node:assert/strict');

module.exports = async function checkProjects(page, api, until) {
  const mainID = '00000000-0000-4000-8000-000000000001';
  const initial = (await api(`/projects/${mainID}/scenes`)).scenes;
  const clickDialog = async (button, value) => {
    page.once('dialog', dialog => dialog.accept(value));
    await page.getByRole('button', { name: button, exact: true }).click();
  };
  await clickDialog('New project', 'Browser isolated project');
  let project;
  await until(async () => {
    project = (await api('/projects')).projects.find(p => p.name === 'Browser isolated project');
    return !!project && await page.locator('#project-picker').inputValue() === project.id;
  });
  await page.locator('#create-scene').waitFor();
  assert.deepEqual((await api(`/projects/${project.id}/scenes`)).scenes, []);
  assert.equal(await page.getByRole('tab', {name:'Browser QA', exact:true}).count(), 0, 'another project scene leaked');
  await page.locator('#create-scene').click();
  await until(async () => (await api(`/projects/${project.id}/scenes`)).scenes.length === 1);
  await page.locator('#scene-name').waitFor();
  await clickDialog('Rename project', 'Browser project renamed');
  await until(async () => (await api('/projects')).projects.some(p => p.id === project.id && p.name === 'Browser project renamed'));
  await until(async () => await page.locator('#project-picker option:checked').textContent() === 'Browser project renamed');
  await page.locator('#project-picker').selectOption(mainID);
  await page.getByRole('tab', {name:'Browser QA', exact:true}).waitFor();
  await page.locator('#project-picker').selectOption(project.id);
  await page.locator('#scene-name').waitFor();
  assert.equal(await page.getByRole('tab', {name:'Browser QA', exact:true}).count(), 0);
  await clickDialog('Delete project');
  await until(async () => !(await api('/projects')).projects.some(p => p.id === project.id));
  await page.getByRole('tab', {name:'Browser QA', exact:true}).waitFor();
  assert.equal(await page.locator('#project-picker').inputValue(), mainID);
  assert.deepEqual((await api(`/projects/${mainID}/scenes`)).scenes, initial, 'deleting another project changed Main');
  console.log('PASS: project create/rename/switch/delete and isolated scene selection');
};
