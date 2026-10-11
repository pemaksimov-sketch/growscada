# Organization и Project: project_id везде
Status: ready-for-human

Первая задача фундамента (слой A). Термины — `CONTEXT.md` (Organization, Project), решение — ADR 0006.

- Агрегат Project: проект — единица изоляции. Tags, Scenes с Widgets, Draft, Revisions, Journal, Devices принадлежат ровно одному Project. Сервер хостит несколько проектов в обеих ролях (Engineering и Runtime node); проекты не ссылаются на сущности друг друга.
- Organization (tenant) владеет Projects и Libraries. Пока только ключ в данных: одна стартовая Organization из seed, без пользователей и входа (User и права — задачи 49, 50).
- `project_id` во всех таблицах проектных сущностей и в их уникальных ограничениях; `organization_id` у Project (и у Library — задача 33).
- Имя Tag уникально внутри Project: естественный ключ — (Project, имя). Обновить ADR 0003, `UNIQUE` в БД, `GET /tags?name=`.
- REST адресует проект: ресурсы проекта — под `/api/v0/projects/{project}/…`. Поднять SchemaVersion API, `make schemas`.
- growctl: манифесты и команды указывают проект (флаг/контекст, как namespace в kubectl). Контракт манифестов — SchemaVersion.
- devicelink и симулятор: пока без Device Token проект задаётся конфигурацией; в задаче 34 проект будет определяться токеном.
- UI: выбор проекта (список проектов, создание, переименование, удаление).
- Миграция dev-БД и seed: существующие данные — в один проект «Main» стартовой Organization.
- Тесты репозиториев (testcontainers), Bruno, `CHANGELOG.md`.

## Comments

- 2026-10-11: Реализованы Project CRUD, стартовая Organization и Main, scope Tags/Scenes/Widgets и outbox/SSE, типизированная фабрика репозиториев, выбор проекта и отдельное сохранение выбора в UI, контекст growctl/manifest 0.2 и симулятора. API 0.3; Library пока общая согласно ADR 0007. Проверена миграция старых данных, одинаковые имена тегов в разных проектах, чужие ID, каскадное удаление и фоновое согласование обоих проектов. Проверки коммита 18db52b: [make test/build, SSE race и браузер](https://github.com/pemaksimov-sketch/growscada/actions/runs/38148466016), [Bruno: 64 запроса, 125/125 тестов](https://github.com/pemaksimov-sketch/growscada/actions/runs/38148465984). Браузер проверяет создание/переименование/переключение/удаление проектов и остановку/перезапуск сервера с живыми потоками.

- 2026-10-04: бывшая задача 30. Отменяет решение Q21 бывшей задачи 36 («один неявный проект»): решено, что project_id нужен сразу везде, включая Runtime node, до появления Journal и Revision — иначе это вторая большая миграция.
- 2026-10-06: из grilling задачи 20 (ADR 0008). Многоагрегатной единицы работы нет, поэтому шов, через который `project_id` попадает в запросы (ADR 0006), решается здесь. Варианты: фабрика репозиториев по области (`ForProject(p)` → `Tags()`/`Scenes()`, `ForLibrary(l)` → `WidgetTypes()` в задаче 33; интерфейсы репозиториев не меняются, забытый проект — ошибка компиляции) или `projectID` в `context.Context` (меньше diff, но ошибка только во время выполнения, политики должны класть проект в ctx сами). Требование в любом случае: строка outbox несёт `project_id`, политика берёт репозитории проекта события.
