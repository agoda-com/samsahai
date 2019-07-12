### Required

- Kubernetes cluster
- Helm
- Helm Operator (fluxcd)


```yaml
# values file structure

image:
  repository: <>
  tag:

dependency1:
  image:
    repository:
    tag:

dependency2:
  image:
    repository:
    tag:

dependency3:
  image:
    repository:
    tag:

```

---

### Project structure

```
<root>
  ├── samsahai.yaml           -> configuration file
  ├── components              -> base values file for each component
  │   ├── <component>.yaml
  │   └── <component>.yaml
  └── env                     -> values file for specific env.
      ├── active
      │   ├── <component>.yaml
      │   └── <component>.yaml
      └── staging
          ├── <component>.yaml
          └── <component>.yaml
```

old
```
<root>
  ├── config.yaml           -> configuration file
  ├── values.yaml           -> base values file
  └── discovery             -> values file for specific env.
      ├── active.yaml
      └── staging.yaml
```

---
