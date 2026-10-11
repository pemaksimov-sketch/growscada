# События Widget несут SceneID, имена событий — в контракте
Status: needs-triage

Появилось в архитектурном ревью 2026-10-05. Сейчас форма живых событий заставляет UI перечитывать больше, чем изменилось. Корректная остановка SSE-соединений — отдельная задача 26.

## Что не так сейчас

- В событиях Widget нет ID Scene. `eventlog.ReloadTargetsFor` на любое событие Widget перечитывает все Scene и все Widget.
- `restapi/event_hub.go` (`eventSourceID`, `eventTag`) разбирает каждое событие через type switch, поэтому каждый новый тип события надо добавлять туда руками.
- Строки типов событий заданы в двух местах: `ui/eventlog/reload.go` и `event_types.go` (отмечено и в задаче 29).

## Что сделать

- Добавить SceneID в события `widget_created`, `widget_updated`, `widget_deleted` (контракт `contract/api/v0`: MINOR-bump SchemaVersion, `make schemas`).
- UI по событию Widget перечитывает только его Scene.
- Типы событий — константы в `contract/api/v0`; UI и restapi используют их, а не свои строки.

## Приёмка

- Изменение Widget в одной Scene не перечитывает остальные Scene (проверить в браузере по сетевым запросам).
- Тест сверяет константы контракта с `eventTypeInfoByType`.
- `make build` и `make test` проходят.


## Comments

- 2026-10-11: реализация проверена на коммите cb34cee: make test, make build, SSE race и браузерные проверки — https://github.com/pemaksimov-sketch/growscada/actions/runs/38134443245. Контейнерные API-тесты — https://github.com/pemaksimov-sketch/growscada/actions/runs/38134443254.
