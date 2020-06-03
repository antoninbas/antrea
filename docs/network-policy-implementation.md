# Antrea Network Policy Implementation

## Introduction

![Antrea NP overview](/docs/assets/antrea-np-overview.png)

Antrea uses a centralized controller (Antrea Controller) which connects to the
k8s apiserver and watches the k8s NetworkPolicies. The Controller then performs
"policy computation", during which the k8s NetworkPolicies are converted to
internal, Antrea-specific, objects which are then distributed (through an
apiserver instance) to the Antrea Agents running on the Nodes. By using a
centralized controller, we avoid overloading the k8s apiserver (by not having
all Agents connect to it individually). We also avoid duplicating computation
tasks across all Agents. By ensuring that each Agent only receives the
information relevant to the Pods running locally on its Node and using
incremental messages when possible, we reduce communication overhead and
simplify the computation logic at each Agent.

This documents describes the different steps of:

1. NetworkPolicy computation at the Controller
2. Selective distribution of computed rules from the Controller to the Agents
3. Local enforcement of computed rules at the Agents by programming OVS

## An end-to-end overview of the implementation

![Antrea NP implementation](/docs/assets/antrea-np-implementation.svg)

1. The Antrea Controller uses the client-go K8s library to implement a K8s
   controller which watches Pods, Namespaces and K8s NetworkPolicies.
2. Events for Pods, Namespaces and NetworkPolicies are used as inputs to compute
   internal objects of types AppliedToGroup, AddressGroup and NetworkPolicy
   (internal representation of a K8s NetworkPolicy), which are then stored into
   in-memory stores. The stores implement Antrea [`storage.Interface`] and are
   implemented using a K8s [Indexer]. The internal objects computed by the
   Antrea controller are described
   [here](#network-policy-computation-in-the-controller). Each of these objects
   include "span" metadata, which is the set of Nodes for which the Antrea Agent
   needs information about this object in order to enforce NetworkPolicies
   correctly.
3. The Antrea Controller runs its own apiserver to handle distribution of
   computed objects to the Antrea Agents. The 3 in-memory stores are wrapped in
   structs which implement the k8s [`rest.Storage`] interface and used as
   backing storage for the apiserver. The objects served by the apiserver are in
   the `networking.antrea.tanzu.vmware.com` API group and version v1beta1 is
   defined
   [here](https://github.com/vmware-tanzu/antrea/blob/master/pkg/apis/networking/v1beta1/types.go). For
   each Antrea Agent connected to the Antrea Controller's apiserver and each of
   the 3 in-memory stores, a [`storeWatcher`] is created, which implements k8s
   [`watch.Interface`].  When an interesting event is generated for any object
   type (AppliedToGroup, AddressGroup, internal NetworkPolicy), the appropriate
   message in the `networking.antrea.tanzu.vmware.com` API is generated and the
   apiserver handles sending it to the client (Antrea Agent). Note that 2
   important factors play a role in the conversion from stored object to API
   message: 1) a stored object may be sharded when sent to an Angent (e.g. for
   an AppliedToGroup, each Agent only needs to be aware of the endpoints in the
   group which are localized to its own Node), and 2) as much as possible the
   Controller tries to send "Patch" messages which only encodes incremental
   changes to the stored object (e.g. new or deleted addresses in an
   AddressGroup).
4. The Antrea Agent uses the client-go k8s library to watch the resources in the
   `networking.antrea.tanzu.vmware.com` API group. For each resource type
   (AppliedToGroup, AddressGroup and NetworkPolicy), a Field Selector is
   provided which filters relevant resources based on the Node name. As
   described above, this ensures that only messages relevant for this Node are
   received from the Antrea Controller.
5. Every resource update received by the watchers is passed along to the
   [`ruleCache`], which stores all the policy rules that need to be enforced
   locally using a k8s [Indexer]. Every time a new update is received, the
   indexer helps locate the impacted rules, which are marked as "dirty" and put
   into a queue for the [`Reconciler`] to handle.
6. The [`Reconciler`] needs to map rules coming from the [`ruleCache`] to
   [`PolicyRules`] that can easily be translated to OF flows. To achieve this,
   it performs 2 major operations. The first one is mapping the "named ports" in
   the rules to their corresponding integer values. Because the same "named
   port" string value (e.g. "http") can map to different integer values based on
   the specification for the Pod under consideration (e.g. 80 for one Pod, 8080
   for another Pod), the same rule received from the [`ruleCache`] can actually
   lead to multiple [`PolicyRules`]. The second operation is the conversion of
   IPBlocks to a set of equivalent non-overlapping CIDRs: in practice, it means
   "subtracting" CIDRs provided with `except` clauses from the main IPBlock
   CIDR. Each rule created by the [`Reconciler`] is identified by a unique ID
   which is used when invoking the appropriate APIs in the [OpenFlow client]
   interface.
7. The [OpenFlow client] is tasked with translating the [`PolicyRules`]
   generated by the [`Reconciler`] to actual flows that can be added to the OVS
   bridge. Most of the complexity of the implementation comes from the fact that
   we leverage [conjunctive matches] in OVS to avoid an explosion in the number
   of flows that need to be programmed. When translating rules to confunctive
   flows, we need to keep these 2 rules in mind: 1) conjunctive flows must not
   overlap with each other at a given priority, and 2) a flow may have multiple
   conjunction actions, with different id values. These means that when
   distinct NetworkPolicy rules apply to the same 

## NetworkPolicy computation in the Controller

The Antrea Controller translates k8s resource (Namespaces, Pods,
NetworkPolicies) to internal objects, defined
[here](https://github.com/vmware-tanzu/antrea/blob/master/pkg/controller/types/networkpolicy.go). We
define 3 top-level objects:

 * `AddressGroup` describes a set of Pods used as source or destination for one
   or more NetworkPolicy rules. Each member of the set includes a Pod reference
   (`namespace/name`), the IPv4 address of the Pod and the "named port" mapping
   (i.e. which name corresponds to which numerical value) for the Pod (as
   obtained from the Pod specification).
* `AppliedToGroup` describes a set of Pods to which one or more NetworkPolicy is
   applied. In practice, this "set" is actually a map from each Node name to the
   subset of Pods running on that Node (when distributing the object to an
   Agent, this enables restricting the information to the Pods running locally
   on that Agent's Node). Each subet of Pods includes the same information (Pod
   reference, IP address and "named port" mapping) as for `AddressGroup`.
* `NetworkPolicy` maps directly to a K8s NetworkPolicy resource, but replaces
   the different Pod Selectors and Namespace Selectors with references (by name)
   the computed `AddressGroup` and `AppliedToGroup` objects. IPBlocks are
   preserved as they are. This object is often referred to as an "internal"
   NetworkPolicy to avoid confusion with the corresponding k8s resource.

Note that when applicable, multiple `NetworkPolicies` can refer to the same
`AppliedToGroups` (e.g., when the k8s NetworkPolicies apply to the same Pods) or
the same `AddressGroups` (e.g., when multiple rules select the same Pods).

## Mapping NetworkPolicies to flows

[`storage.Interface`]: https://github.com/vmware-tanzu/antrea/blob/master/pkg/apiserver/storage/interfaces.go
[Indexer]: https://godoc.org/k8s.io/client-go/tools/cache#Indexer
[`rest.Storage`]: https://godoc.org/k8s.io/apiserver/pkg/registry/rest#Storage
[`storeWatcher`]: https://github.com/vmware-tanzu/antrea/blob/master/pkg/apiserver/storage/ram/watch.go
[`watch.Interface`]: https://godoc.org/k8s.io/apimachinery/pkg/watch#Interface
[`ruleCache`]: https://github.com/vmware-tanzu/antrea/blob/master/pkg/agent/controller/networkpolicy/cache.go
[`Reconciler`]: https://github.com/vmware-tanzu/antrea/blob/master/pkg/agent/controller/networkpolicy/reconciler.go
[`PolicyRules`]: https://github.com/vmware-tanzu/antrea/blob/master/pkg/agent/types/networkpolicy.go
[OpenFlow client]: https://github.com/vmware-tanzu/antrea/blob/master/pkg/agent/openflow/client.go
[conjunctive matches]: http://www.openvswitch.org/support/dist-docs/ovs-fields.7.txt
