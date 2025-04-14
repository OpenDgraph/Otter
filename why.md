
# Why this Software?

Well, Dgraph is an amazing database. It does everything it promises and it does it well. However, if you're not a skilled database admin, you might run into some issues.

Dgraph is inherently concurrent. The way it handles queries is parallelized and very efficient. But problems arise when, unintentionally, all your clients start hitting a single Dgraph instance. This can lead to overload and eventually cause an OOM error. Sure, it recovers gracefully, but this could have been avoided with proper best practices. The issue is: Dgraph doesn't enforce these best practices out of the box for production use.

That’s why I created **Otter**.

Otter aims to implement several features I’ve always wanted to see in Dgraph. But which were never accepted into the core codebase. The reasons varied: some were out of scope (understandably), others lacked available contributors to maintain them, and some were simply seen as too complex. I’m not the CEO, CTO, or a founder - I can’t impose my vision. So the best way forward was to build something like a framework. Hence, **Otter**.

Here, I’ll be implementing a lot of features. Most of them optional. You’ll be able to write DQL queries against a GraphQL-style graph model, without worrying about the internal details. Otter will handle the conversion, transpiling your raw DQL into the format required by Dgraph’s internal GraphQL layer.

Beyond that, I may implement transpilers for Cypher and other query languages to make integration easier. I’m also interested in adding strategies to make Dgraph more atomic enabling entity-level sharding, rather than just predicate-level sharding. Of course, everything comes with trade-offs, and you'll choose how much you're willing to sacrifice (usually in terms of storage) for the huge benefits of sharding.

At the moment, I’ve only implemented a basic proxy/gateway with load balancing. But much more is on the way.

#  My notes

Nothing below is written in stone. Everything can change. Just by exploring ideas.

## DRAFT General IDEA.  


note 01: This is an opinionated framework — graph structure is suggested and enforced to ensure consistency and optimize performance across all deployments.

note 02: Prefix-based predicate sharding allows region-specific tablets to remain small and local, reducing index scan time and boosting localized query throughput.

# Dgraph Framework Ideas

## Type Sharding

**Idea**: Create a Dgraph framework that splits a type into several smaller types, enhancing distributed sharding possibilities.

Example:
- Instead of having a type person, extend it by country/region: type.person.br, type.person.us, etc.
- This could split person into ~190 parts, making data distribution easier across clusters.

## Semantic Queries

**Idea**: Use semantic patterns like dtype.person.us.ca.sf to query people from San Francisco, CA. 
- Leverage Dgraph's semantic system.
- Embed these predicate patterns.
- Semantic search maps natural queries like "Give me all the people from San Francisco CA" to UIDs/predicates.

## UID Reservation

- Reserve **100,000 UIDs** up front.
- First **10 reserved** for internal framework use.
- Remaining for **graph creation**.
- Enables interconnection between graphs.
- Establishes a consistent **starting point**.

## Predefined Graph Structures

**Idea**: Hierarchical structures:
- Continents → Countries → Regions → Cities → Neighborhoods → Streets
- Delivered in a downloadable .p file.
- Saves setup time and standardizes UIDs.
- Example: UID 0x342F32 is predefined for San Francisco, CA.
- Can cache UID to avoid repeated lookups.

## Chemistry Schema

**Idea**: Define UIDs for the periodic table.
- Build a chemistry-focused graph.
- Could support **academic/scientific** research.

## Ontological Schema System

**Goal**: Allow schemas to have built-in ontologies.

Example:

```graphql
schema for Users {
  type User is Person {
    name: String @index(hash) # inherited unless overridden
  }
  type Login belongs User { ... }
  type Admin extends User { ... }
}

schema for Products {
  type Product { ... }
}

schema for Companies {
  type Company { ... }
}
```

Resulting predicates:

```graphql
g.11.User.name: String @index(hash)
g.11.Login.name: String @index(hash)
g.11.Admin # inherits name from User
```

## Region-Based Indexing

**Advanced Sharding**:

```graphql
type User is Person @By(region)
```

Generates:
```graphql
g.11.Europe.User.name: String @index(hash)
g.11.America.User.name: String @index(hash)
g.11.Africa.User.name: String @index(hash)
```

```graphql
type User is Person @By(region.country)
```

Generates:
```graphql
g.11.Europe.fr.User.name: String @index(hash)
g.11.America.us.User.name: String @index(hash)
g.11.Africa.ug.User.name: String @index(hash)
g.11.Asia.jp.User.name: String @index(hash)
```

```graphql
type User is Person @By(region.country.city)
```

Generates:
```graphql
g.11.Europe.fr.paris.User.name: String @index(hash)
g.11.America.us.sf.User.name: String @index(hash)
g.11.Africa.ug.kla.User.name: String @index(hash)
g.11.Asia.jp.tyo.User.name: String @index(hash)
```

## UPSERT Support for Facets

**Idea**: Add native support for upsert operations on facets.

## Query Decorators

**Goal**: Avoid confusing queries via decorators.

Example:
```graphql
Query @(graph:   "g11",    # pick a Graph name, uid or number
        region:  "Europe", # Set the region and others
        country: "France", # Next time this traversed
        city:    "Paris"   # tree will be cached
        )
{
  q(func: Type(User)) {
    name
  }
}
```

This query targets the graph g11, scoped by geographic filters (Europe → France → Paris), and retrieves name fields from all nodes of type User. This traversal path will be cached for faster access in future queries.