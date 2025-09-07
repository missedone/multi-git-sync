Use cases
===

## Sync AirFlow DAGs from multiple git repos
A key use case for `multi-git-sync` is syncing Apache AirFlow DAGs from multiple Git repositories. While the official AirFlow Helm chart only supports pulling DAGs from a single repository, `multi-git-sync` offers a robust solution to this limitation. This addresses [an ongoing discussion](https://github.com/apache/airflow/discussions/19381) and [a workaround](https://blog.devops.dev/airflow-gitsync-with-multiple-repos-or-branches-6d66c6a8aa24) within the AirFlow community regarding multi-repository support.

### How It Works with AirFlow

`multi-git-sync` can be deployed as a sidecar container alongside the AirFlow webserver. It syncs DAGs from multiple Git repos to a shared volume, which is then accessible by the webserver.

Here is a configuration snippet from the `values.yaml` of the AirFlow Helm chart, showing how `multi-git-sync` is used as a sidecar container to sync DAGs to a shared volume.

```yaml
# values.yaml
webserver:
  enabled: true
  extraVolumes:
    - name: git-sync-conf-vol
      configMap:
        name: multi-git-sync-cm
    - name: git-sync-DAGs
      persistentVolumeClaim:
        claimName: AirFlow-DAGs-pvc
  extraContainers:
    - name: multi-git-sync
      image: ghcr.io/missedone/multi-git-sync:0.1.1
      args:
        - "-config=/opt/git-sync/config.yaml"
      volumeMounts:
        - name: git-sync-DAGs
          mountPath: /git
        - name: git-sync-conf-vol
          mountPath: /opt/git-sync/
      envFrom:
        - secretRef:
            name: git-credentials
```

The tool's settings are defined in a `ConfigMap`. For example, you can configure it to sync the `DAGs` subpath from one repo to `/git/DAGs1` and the `DAGs` subpath from another repo to `/git/DAGs2` on the webserver.

```yaml
# config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: multi-git-sync-cm
data:
  config.yaml: |-
    repos:
      - url: 'https://github.com/missedone/AirFlow-example-DAGs1.git'
        branch: main
        subPath: DAGs
        auth:
          user: {{ getEnv "GIT_SYNC_USERNAME" }}
          accessToken: {{ getEnv "GIT_SYNC_PASSWORD" }}
        destDir: /git/DAGs1
        schedule: '*/1 * * * *'
        depth: 1
      - url: 'https://github.com/missedone/AirFlow-example-DAGs2.git'
        branch: main
        subPath: DAGs
        auth:
          user: {{ getEnv "GIT_SYNC_USERNAME" }}
          accessToken: {{ getEnv "GIT_SYNC_PASSWORD" }}
        destDir: /git/DAGs2
        schedule: '*/1 * * * *'
        depth: 1
```

besides, you may noticed that in the sample `config.yaml` we use the git sparse checkout with shallow clone which can speedup the sync when only need to get the small part of the large git repo.
