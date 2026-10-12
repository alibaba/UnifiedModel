import org.apache.tinkerpop.gremlin.tinkergraph.structure.TinkerGraph
import org.apache.tinkerpop.gremlin.structure.io.IoCore
import org.apache.tinkerpop.gremlin.structure.T
import org.mindrot.jbcrypt.BCrypt

def graph = TinkerGraph.open()
graph.addVertex(T.label, 'user', 'username', 'umodel-test-user',
    'password', BCrypt.hashpw('umodel-test-password', BCrypt.gensalt()))
graph.io(IoCore.graphson()).writeGraph('/state/credentials.json')
graph.close()
