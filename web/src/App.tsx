import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import DiscoveryPage from './pages/DiscoveryPage'
import ElementsPage from './pages/ElementsPage'
import ClustersPage from './pages/ClustersPage'
import RulesPage from './pages/RulesPage'
import RuleEditorPage from './pages/RuleEditorPage'

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<DiscoveryPage />} />
        <Route path="/elements" element={<ElementsPage />} />
        <Route path="/clusters" element={<ClustersPage />} />
        <Route path="/rules" element={<RulesPage />} />
        <Route path="/rules/new" element={<RuleEditorPage />} />
        <Route path="/rules/:id" element={<RuleEditorPage />} />
      </Routes>
    </Layout>
  )
}

export default App
