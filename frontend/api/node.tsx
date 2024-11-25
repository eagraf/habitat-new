import axios from 'axios';
import Cookies from 'js-cookie';
import * as api from '../types/api';
import * as node from '../types/node';

export const getNode = async (): Promise<api.GetNodeResponse> => {
    try {
        const accessToken = Cookies.get('access_token');
        if (!accessToken) {
            throw new Error('No access token found');
        }

        const response = await axios.get(`${window.location.origin}/habitat/api/node`, {
            headers: {
                'Authorization': `Bearer ${accessToken}`,
                'Content-Type': 'application/json',
            },
        });

        return response.data;
    } catch (error) {
        console.error('Error fetching node data:', error);
        throw error;
    }
};

export const installApp = async (appInstallation: api.PostAppRequest) => {
    try {
        const accessToken = Cookies.get('access_token');
        if (!accessToken) {
            throw new Error('No access token found');
        }

        const response = await axios.post(
            `${window.location.origin}/habitat/api/node/users/0/apps`,
            appInstallation,
            {
                headers: {
                    'Authorization': `Bearer ${accessToken}`,
                    'Content-Type': 'application/json',
                },
            }
        );

        return response.data;
    } catch (error) {
        console.error('Error installing app:', error);
        throw error;
    }
};

export const upgradeApp = async (upgradeRequest: api.PostUpgradeAppRequest) => {
    try {
        const accessToken = Cookies.get('access_token');
        if (!accessToken) {
            throw new Error('No access token found');
        }

        const response = await axios.post(
            `${window.location.origin}/habitat/api/node/users/0/apps/${upgradeRequest.app_installation?.id}/upgrade`,
            upgradeRequest,
            {
                headers: { 'Authorization': `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
            }
        );

        return response.data;
    } catch (error) {
        console.error('Error upgrading app:', error);
        throw error;
    }
};

export const getAvailableApps = async (): Promise<api.PostAppRequest[]> => {
    try {
        const response = await axios.get(`${window.location.origin}/habitat/api/app_store/available_apps`);
        return response.data;
    } catch (error) {
        console.error('Error fetching available apps:', error);
        throw error;
    }
};


// Helpers


export const getWebApps = async (): Promise<any[]> => {
    try {
        const nodeState = await getNode();
        const appInstallations = Object.values(nodeState.state.app_installations || {});

        const webApps = appInstallations
            .filter((app: any) => app.driver === 'web')
            .map((app: any) => ({
                id: app.id,
                name: app.name,
                driver: app.driver,
            }));

        const reverseProxyRules = nodeState.state.reverse_proxy_rules || {};

        return webApps.map(app => {
            const appRules = Object.values(reverseProxyRules).filter((rule: any) => rule.app_id === app.id);
            const fileRule = appRules.find((rule: any) => rule.type === 'file');

            return {
                ...app,
                url: fileRule ? fileRule.matcher : undefined
            };
        });
    } catch (error) {
        console.error('Error fetching web apps:', error);
        throw error;
    }
};

export type AppWithInstallStatus = {
    // Store this separately, because appRequest will not have the ID set
    appID: string | undefined;
    appRequest: api.PostAppRequest;
    installedApp?: node.AppInstallationState;
    installed: boolean;
    needsUpdate: boolean;
}


export const getAvailableAppsWithInstallStatus = async (): Promise<AppWithInstallStatus[]> => {
    try {
        const availableApps = await getAvailableApps();
        const nodeState = await getNode();
        const installedApps = Object.values(nodeState.state.app_installations || {});

        return availableApps.map(app => {
            const installedApp = installedApps.find((installedApp: any) => {
                return installedApp.name === app.app_installation?.name
            });

            if (installedApp) {
                return {
                    appID: installedApp.id,
                    appRequest: app,
                    installedApp: installedApp,
                    installed: true,
                    needsUpdate: installedApp.version < app.app_installation!.version,
                };
            }

            return {
                appID: undefined,
                appRequest: app,
                installedApp: undefined,
                installed: false,
                needsUpdate: false,
            };
        });
    } catch (error) {
        console.error('Error fetching available apps with install status:', error);
        throw error;
    }
};
