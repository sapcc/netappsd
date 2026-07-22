apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app: netappsd-worker
spec:
  selector:
    matchLabels:
      app: netappsd-worker
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  template:
    metadata:
      labels:
        app: netappsd-worker
    spec:
      serviceAccountName: netappsd
      containers:
        - name: poller
          image: keppel.eu-de-1.cloud.sap/ccloud-dockerhub-mirror/rahulguptajss/harvest:23.11.0-1
          imagePullPolicy: IfNotPresent
          command: ["/busybox/sh", "/opt/harvest/start_poller.sh"]
          ports:
            - name: metrics
              containerPort: 13000
          resources:
            requests:
              cpu: 100m
              memory: 100Mi
            limits:
              cpu: 500m
              memory: 500Mi
          volumeMounts:
            - name: netappsd-harvest
              mountPath: /opt/harvest/start_poller.sh
              subPath: start_poller.sh
            - name: netappsd-harvest
              mountPath: /opt/harvest/conf/rest/custom.yaml
              subPath: rest.custom.yaml
            - name: netappsd-harvest
              mountPath: /opt/harvest/conf/rest/9.12.0/custom_node.yaml
              subPath: rest.custom_node.yaml
            - name: netappsd-harvest
              mountPath: /opt/harvest/conf/rest/9.12.0/custom_volume.yaml
              subPath: rest.custom_volume.yaml
            - name: netappsd-harvest
              mountPath: /opt/harvest/conf/rest/limited.yaml
              subPath: rest.limited.yaml
            - name: netappsd-harvest
              mountPath: /opt/harvest/conf/restperf/limited.yaml
              subPath: restperf.limited.yaml
            - name: shared
              mountPath: /opt/harvest/shared
        - name: netappsd-worker
          image: keppel.eu-de-1.cloud.sap/ccloud/netappsd-amd64:latest
          imagePullPolicy: Always
          command: ["/app/netappsd"]
          args:
            - worker
            - --master-url
            - http://netappsd-master.netapp-exporters.svc:8080
            - --template-file
            - /app/harvest.yaml.tpl
            - --output-file
            - /app/shared
            - --debug
          resources:
            requests:
              cpu: 100m
              memory: 100Mi
            limits:
              cpu: 500m
              memory: 500Mi
          env:
            - name: NETAPP_USERNAME
              valueFrom:
                secretKeyRef:
                  name: netappsd
                  key: netappUsername
            - name: NETAPP_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: netappsd
                  key: netappPassword
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          volumeMounts:
            - name: netappsd-harvest
              mountPath: /app/harvest.yaml.tpl
              subPath: harvest.yaml.tpl
            - name: shared
              mountPath: /app/shared
      volumes:
        - name: netappsd-harvest
          configMap:
            name: netappsd-harvest
        - name: shared
          emptyDir: {}
