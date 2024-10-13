'use client'

import React, { useState, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import withAuth from '@/components/withAuth';
import { useAuth } from '@/components/authContext';
import { getNode } from '@/api/node';
import ReverseProxyRuleList from '@/components/reverseProxyRuleList';
import AppList from '@/components/appList';

const ServerPage: React.FC = () => {
    const router = useRouter();
    const searchParams = useSearchParams();
    const { logout } = useAuth();
    const [activeTab, setActiveTab] = useState(searchParams.get('tab') || 'apps');
    const [nodeData, setNodeData] = useState(null);

    useEffect(() => {
        const tab = searchParams.get('tab');
        if (tab) {
            setActiveTab(tab);
        }
    }, [searchParams]);

    const handleTabChange = (tab: string) => {
        setActiveTab(tab);
        router.push(`/server?tab=${tab}`);
    };

    const tabs = ['Apps', 'Reverse Proxy', 'Users'];

    useEffect(() => {
        const fetchNodeData = async () => {
            try {
                const data = await getNode();
                setNodeData(data);
            } catch (error) {
                console.error('Error fetching node data:', error);
            }
        };

        fetchNodeData();
    }, []);

    const renderTabContent = () => {
        switch (activeTab) {
            case 'apps':
                const processesArray = Object.values(nodeData!.state.processes);
                const appsArray = Object.values(nodeData!.state.app_installations);
                const proxyRulesArray = Object.values(nodeData!.state.reverse_proxy_rules);
                return <AppList apps={appsArray} processes={processesArray} reverseProxyRules={proxyRulesArray} />;
            case 'reverseproxy':
                console.log(nodeData);
                console.log(nodeData!.state.reverse_proxy_rules);
                return <ReverseProxyRuleList rules={nodeData!.state.reverse_proxy_rules} />;
            case 'users':
                return <div>Users content goes here</div>;
            default:
                return null;
        }
    };

    return (
        <main className="flex flex-col w-full min-h-screen p-8">
            <h1 className="text-3xl font-bold mb-6">Habitat Node State</h1>
            
            <div className="flex mb-6">
                {tabs.map((tab) => (
                    <button
                        key={tab}
                        className={`px-4 py-2 mr-2 rounded-t-lg ${
                            activeTab === tab.toLowerCase().replace(' ', '')
                                ? 'bg-blue-500 text-white'
                                : 'bg-gray-200'
                        }`}
                        onClick={() => handleTabChange(tab.toLowerCase().replace(' ', ''))}
                    >
                        {tab}
                    </button>
                ))}
            </div>

            <div className="bg-white p-6 rounded-lg shadow">
                {nodeData ? renderTabContent() : <p>Loading node data...</p>}
            </div>
        </main>
    );
};

export default withAuth(ServerPage);