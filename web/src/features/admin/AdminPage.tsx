import { useSearchParams } from 'react-router';
import { ru } from '@/shared/i18n/ru';
import { PageHeader, Tabs, TabsContent, TabsList, TabsTrigger, useDocumentTitle } from '@/shared/ui';
import { RulesTab } from './RulesTab';
import { CategoriesTab } from './CategoriesTab';
import { UsersTab } from './UsersTab';

type AdminTab = 'routing' | 'categories' | 'users';
const TABS: AdminTab[] = ['routing', 'categories', 'users'];

export default function AdminPage() {
  const [sp, setSp] = useSearchParams();
  const tab = (TABS.includes(sp.get('tab') as AdminTab) ? sp.get('tab') : 'routing') as AdminTab;
  useDocumentTitle(`${ru.admin[tab]} · ${ru.admin.title}`);
  return (
    <div>
      <PageHeader title={ru.admin.title} lead={ru.admin.lead} />
      <Tabs value={tab} onValueChange={(v) => setSp(v === 'routing' ? {} : { tab: v }, { replace: true })}>
        <TabsList aria-label={ru.admin.tabs} className="mb-6">
          {TABS.map((t) => (
            <TabsTrigger key={t} value={t}>
              {ru.admin[t]}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="routing">
          <RulesTab />
        </TabsContent>
        <TabsContent value="categories">
          <CategoriesTab />
        </TabsContent>
        <TabsContent value="users">
          <UsersTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
