# Managing Library WidgetTypes with growctl

`growctl widget-type apply -f widget-type.yaml` creates or updates the named WidgetType in the Library Draft selected by `metadata.library`. Use the Library UUID shown by `GET /api/v0/libraries`. Applying a manifest changes only the Draft; publish a Library Release separately, then pin that release to a project.

```yaml
apiVersion: growscada/v0.3
kind: WidgetType
metadata:
  name: Gauge
  library: 00000000-0000-4000-8000-000000000003
spec:
  htmlTemplate: '<div class="value"></div>'
  script: 'element.textContent = inputs.value ?? "—";'
  scriptLanguage: javascript
  defaultWidth: 120
  defaultHeight: 60
  inputPorts:
    - name: value
      description: Process value
      typeHint: number
```

The manifest API version is strict. Unknown fields, an invalid Library UUID, duplicate names in one Library, and invalid input ports fail before any changes are sent.
