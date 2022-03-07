const uuid = require('uuid');
const {
  DirectorConfig,
  DirectorClient,
  addScenarioInCompass,
  assignRuntimeToScenario,
  unregisterKymaFromCompass,
} = require('../compass');
const {
  KEBConfig,
  KEBClient,
  provisionSKR,
  deprovisionSKR,
} = require('../kyma-environment-broker');
const {GardenerConfig, GardenerClient} = require('../gardener');
const {
  debug,
  genRandom,
  initializeK8sClient,
  printRestartReport,
  getContainerRestartsForAllNamespaces,
  getEnvOrThrow,
  switchDebug,
} = require('../utils');
const {
  ensureCommerceMockWithCompassTestFixture,
  checkInClusterEventDelivery,
  checkFunctionResponse,
  sendLegacyEventAndCheckResponse,
} = require('../test/fixtures/commerce-mock');
const {
  checkServiceInstanceExistence,
  ensureHelmBrokerTestFixture,
} = require('../upgrade-test/fixtures/helm-broker');
const {
  cleanMockTestFixture,
} = require('../test/fixtures/commerce-mock');
const {
  KCPConfig,
  KCPWrapper,
} = require('../kcp/client');
const {
  saveKubeconfig,
} = require('../skr-svcat-migration-test/test-helpers');

describe('SKR-Upgrade-test', function() {
  switchDebug(on = true);
  const keb = new KEBClient(KEBConfig.fromEnv());
  const gardener = new GardenerClient(GardenerConfig.fromEnv());

  const suffix = genRandom(4);
  const runtimeName = getEnvOrThrow('RUNTIME_NAME');
  const scenarioName = `test-${suffix}`;
  const instanceId = getEnvOrThrow('INSTANCE_ID');
  const subAccountID = keb.subaccountID;
  const kymaOverridesVersion = process.env.KYMA_OVERRIDES_VERSION || '';
  const kymaVersion = getEnvOrThrow('KYMA_VERSION');

  debug(
      `PlanID ${getEnvOrThrow('KEB_PLAN_ID')}`,
      `SubAccountID ${subAccountID}`,
      `instanceId ${instanceId}`,
      `Scenario ${scenarioName}`,
      `Runtime ${runtimeName}`,
      // `Application ${appName}`,
  );

  const kcp = new KCPWrapper(KCPConfig.fromEnv());

  this.timeout(60 * 60 * 1000 * 3); // 3h
  this.slow(5000);

  const provisioningTimeout = 1000 * 60 * 60; // 1h
  const deprovisioningTimeout = 1000 * 60 * 30; // 30m

  const customParams = {'kymaVersion': kymaVersion};
  if (kymaOverridesVersion) {
    customParams['overridesVersion'] = kymaOverridesVersion;
  }

  let skr;

  // SKR Provisioning

  it(`Perform kcp login`, async function() {
    const version = await kcp.version([]);
    debug(version);

    await kcp.login();
    // debug(loginOutput)
  });

  it(`Provision SKR with ID ${instanceId}`, async function() {
    console.log(`Provisioning SKR with version ${kymaVersion}`);
    const customParams = {
      'kymaVersion': kymaVersion,
    };
    debug(`Provision SKR with Custom Parameters ${JSON.stringify(customParams)}`);
    skr = await provisionSKR(
        keb,
        kcp,
        gardener,
        instanceId,
        runtimeName,
        null,
        null,
        customParams,
        provisioningTimeout);
  });

  it(`Should get Runtime Status after provisioning`, async function() {
    const runtimeStatus = await kcp.getRuntimeStatusOperations(instanceId);
    console.log(`\nRuntime status: ${runtimeStatus}`);
    await kcp.reconcileInformationLog(runtimeStatus);
  });

  it(`Should save kubeconfig for the SKR to ~/.kube/config`, async function() {
    saveKubeconfig(skr.shoot.kubeconfig);
  });

  // it('Should initialize K8s client', async function() {
  //   await initializeK8sClient({kubeconfig: skr.shoot.kubeconfig});
  // });

  // Upgrade Test Praparation
  // const director = new DirectorClient(DirectorConfig.fromEnv());
  // const withCentralAppConnectivity = (process.env.WITH_CENTRAL_APP_CONNECTIVITY === 'true');
  // const testNS = 'test';

  // it('Assign SKR to scenario', async function() {
  //   await addScenarioInCompass(director, scenarioName);
  //   await assignRuntimeToScenario(director, skr.shoot.compassID, scenarioName);
  // });

  // it('CommerceMock test fixture should be ready', async function() {
  //   await ensureCommerceMockWithCompassTestFixture(director,
  //       appName,
  //       scenarioName,
  //       'mocks',
  //       testNS,
  //       withCentralAppConnectivity);
  // });

  // it('Helm Broker test fixture should be ready', async function() {
  //   await ensureHelmBrokerTestFixture(testNS).catch((err) => {
  //     console.dir(err); // first error is logged
  //     return ensureHelmBrokerTestFixture(testNS);
  //   });
  // });

  // Cleanup
  const skipCleanup = getEnvOrThrow('SKIP_CLEANUP');
  if (skipCleanup === 'FALSE') {
    // it('Unregister Kyma resources from Compass', async function() {
    //   await unregisterKymaFromCompass(director, scenarioName);
    // });

    // it('Test fixtures should be deleted', async function() {
    //   await cleanMockTestFixture('mocks', testNS, true);
    // });

    it('Deprovision SKR', async function() {
      try {
        await deprovisionSKR(keb, kcp, instanceId, deprovisioningTimeout);
      } finally {
        const runtimeStatus = await kcp.getRuntimeStatusOperations(instanceId);
        await kcp.reconcileInformationLog(runtimeStatus);
      }
    });
  }
});
