
# Why this Software?

Well, Dgraph is an amazing database. It does everything it promises and it does it well. However, if you're not a skilled database admin, you might run into some issues.

Dgraph is inherently concurrent. The way it handles queries is parallelized and very efficient. But problems arise when, unintentionally, all your clients start hitting a single Dgraph instance. This can lead to overload and eventually cause an OOM error. Sure, it recovers gracefully, but this could have been avoided with proper best practices. The issue is: Dgraph doesn't enforce these best practices out of the box for production use.

That’s why I created **Otter**.

Otter aims to implement several features I’ve always wanted to see in Dgraph. But which were never accepted into the core codebase. The reasons varied: some were out of scope (understandably), others lacked available contributors to maintain them, and some were simply seen as too complex. I’m not the CEO, CTO, or a founder - I can’t impose my vision. So the best way forward was to build something like a framework. Hence, **Otter**.

Here, I’ll be implementing a lot of features. Most of them optional. You’ll be able to write DQL queries against a GraphQL-style graph model, without worrying about the internal details. Otter will handle the conversion, transpiling your raw DQL into the format required by Dgraph’s internal GraphQL layer.

Beyond that, I may implement transpilers for Cypher and other query languages to make integration easier. I’m also interested in adding strategies to make Dgraph more atomic enabling entity-level sharding, rather than just predicate-level sharding. Of course, everything comes with trade-offs, and you'll choose how much you're willing to sacrifice (usually in terms of storage) for the huge benefits of sharding.

At the moment, I’ve only implemented a basic proxy/gateway with load balancing. But much more is on the way.
