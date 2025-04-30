import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';

const protoFileDir = './proto';
const protoFileName = 'jobs.proto';

const serviceName = 'demo.Jobs';
const methodName = 'GetJob';

export let options = {
  stages: [
    { duration: '30s', target: 30 },
    { duration: '1m', target: 60 },
    { duration: '20s', target: 0 },
  ],
  thresholds: {
    grpc_req_duration: ['p(95)<300'],
  },
};

const client = new grpc.Client();

client.load([protoFileDir], protoFileName);

export default () => {
  client.connect('0.0.0.0:8080', {
    plaintext: true,
    timeout: 10000
  });

  const requestData = {
    "id": 1
  };

  const response = client.invoke(`/${serviceName}/${methodName}`, requestData);

  check(response, {
    'status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  client.close();

  sleep(0.5);
};
