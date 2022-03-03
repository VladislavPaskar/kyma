const uuid = require('uuid');
const {
  provisionSKR,
  saveKubeconfig,
} = require('../../kyma-environment-broker');

const {
  KCPConfig,
  KCPWrapper,
} = require('../../kcp/client');

const {
} = require('../../skr-test');

const {
  getEnvOrThrow,
  debug,
} = require('../../utils');
const {genRandom, switchDebug} = require('../utils');
const {KEBClient, KEBConfig} = require('../kyma-environment-broker');
const {GardenerClient, GardenerConfig} = require('../gardener');

const instanceId = process.env.INSTANCE_ID || uuid.v4();


describe('Provision SKR cluster', function() {
  switchDebug(on = true);
  const keb = new KEBClient(KEBConfig.fromEnv());
  const gardener = new GardenerClient(GardenerConfig.fromEnv());

  const suffix = genRandom(4);
  const appName = `app-${suffix}`;
  const runtimeName = `kyma-${suffix}`;
  const scenarioName = `test-${suffix}`;
  const instanceID = uuid.v4();
  const subAccountID = keb.subaccountID;

  debug(
      `PlanID ${getEnvOrThrow('KEB_PLAN_ID')}`,
      `SubAccountID ${subAccountID}`,
      `InstanceID ${instanceID}`,
      `Scenario ${scenarioName}`,
      `Runtime ${runtimeName}`,
      `Application ${appName}`,
  );

  // debug(
  //   `KEB_HOST: ${getEnvOrThrow("KEB_HOST")}`,
  //   `KEB_CLIENT_ID: ${getEnvOrThrow("KEB_CLIENT_ID")}`,
  //   `KEB_CLIENT_SECRET: ${getEnvOrThrow("KEB_CLIENT_SECRET")}`,
  //   `KEB_GLOBALACCOUNT_ID: ${getEnvOrThrow("KEB_GLOBALACCOUNT_ID")}`,
  //   `KEB_SUBACCOUNT_ID: ${getEnvOrThrow("KEB_SUBACCOUNT_ID")}`,
  //   `KEB_USER_ID: ${getEnvOrThrow("KEB_USER_ID")}`,
  //   `KEB_PLAN_ID: ${getEnvOrThrow("KEB_PLAN_ID")}`
  // );

  // debug(
  //   `COMPASS_HOST: ${getEnvOrThrow("COMPASS_HOST")}`,
  //   `COMPASS_CLIENT_ID: ${getEnvOrThrow("COMPASS_CLIENT_ID")}`,
  //   `COMPASS_CLIENT_SECRET: ${getEnvOrThrow("COMPASS_CLIENT_SECRET")}`,
  //   `COMPASS_TENANT: ${getEnvOrThrow("COMPASS_TENANT")}`,
  // )

  // debug(
  //   `KCP_TECH_USER_LOGIN: ${KCP_TECH_USER_LOGIN}\n`,
  //   `KCP_TECH_USER_PASSWORD: ${KCP_TECH_USER_PASSWORD}\n`,
  //   `KCP_OIDC_CLIENT_ID: ${KCP_OIDC_CLIENT_ID}\n`,
  //   `KCP_OIDC_CLIENT_SECRET: ${KCP_OIDC_CLIENT_SECRET}\n`,
  //   `KCP_KEB_API_URL: ${KCP_KEB_API_URL}\n`,
  //   `KCP_OIDC_ISSUER_URL: ${KCP_OIDC_ISSUER_URL}\n`
  // )

  // Credentials for KCP ODIC Login

  // process.env.KCP_TECH_USER_LOGIN    =
  // process.env.KCP_TECH_USER_PASSWORD =
  process.env.KCP_OIDC_ISSUER_URL = 'https://kymatest.accounts400.ondemand.com';
  // process.env.KCP_OIDC_CLIENT_ID     =
  // process.env.KCP_OIDC_CLIENT_SECRET =
  process.env.KCP_KEB_API_URL = 'https://kyma-env-broker.cp.dev.kyma.cloud.sap';
  process.env.KCP_GARDENER_NAMESPACE = 'garden-kyma-dev';
  process.env.KCP_MOTHERSHIP_API_URL = 'https://mothership-reconciler.cp.dev.kyma.cloud.sap/v1';
  process.env.KCP_KUBECONFIG_API_URL = 'https://kubeconfig-service.cp.dev.kyma.cloud.sap';

  const kcp = new KCPWrapper(KCPConfig.fromEnv());

  const kymaVersion = getEnvOrThrow('KYMA_VERSION');

  this.timeout(60 * 60 * 1000 * 3); // 3h
  this.slow(5000);

  const provisioningTimeout = 1000 * 60 * 60; // 1h

  let skr;

  // SKR Provisioning

  it(`Perform kcp login`, async function() {
    const version = await kcp.version([]);
    debug(version);

    await kcp.login();
    // debug(loginOutput)
  });

  it(`Provision SKR with ID ${instanceID}`, async function() {
    console.log(`Provisioning SKR with version ${kymaVersion}`);
    const customParams = {
      'kymaVersion': kymaVersion,
    };
    debug(`Provision SKR with Custom Parameters ${JSON.stringify(customParams)}`);
    skr = await provisionSKR(keb,
        kcp,
        gardener,
        instanceID,
        runtimeName,
        null,
        null,
        customParams,
        provisioningTimeout);
  });

  describe('Check provisioned SKR', function() {
    it('Should get Runtime Status after provisioning', async function() {
      const runtimeStatus = await kcp.getRuntimeStatusOperations(instanceId);
      debug(`\nRuntime status: ${runtimeStatus}`);
    });
    it(`Should save kubeconfig for the SKR to ~/.kube/config`, async function() {
      await saveKubeconfig(skr.shoot.kubeconfig);
    });
  });
});
