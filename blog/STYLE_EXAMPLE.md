CNI Migration Made Simple
=========================

Our 3-Phase Strategy for Migrating the CNI Stack in Atmos from AWS VPC CNI + Calico to Cilium
---------------------------------------------------------------------------------------------

[![Guidewire Engineering Team](https://miro.medium.com/v2/resize:fill:64:64/1*5_SyBmjLbylMlEEDfxnm6w.png)](https://medium.com/@guidewire-engineering?source=post_page---byline--ed5f80783537---------------------------------------)

[Guidewire Engineering Team](https://medium.com/@guidewire-engineering?source=post_page---byline--ed5f80783537---------------------------------------)

5 min read

·

Dec 6, 2024

[nameless link](https://medium.com/m/signin?actionUrl=https%3A%2F%2Fmedium.com%2F_%2Fvote%2Fguidewire-engineering-blog%2Fed5f80783537&operation=register&redirect=https%3A%2F%2Fmedium.com%2Fguidewire-engineering-blog%2Fcni-migration-made-simple-ed5f80783537&user=Guidewire+Engineering+Team&userId=745a05261ac2&source=---header_actions--ed5f80783537---------------------clap_footer------------------)

--

[nameless link](https://medium.com/m/signin?actionUrl=https%3A%2F%2Fmedium.com%2F_%2Fbookmark%2Fp%2Fed5f80783537&operation=register&redirect=https%3A%2F%2Fmedium.com%2Fguidewire-engineering-blog%2Fcni-migration-made-simple-ed5f80783537&source=---header_actions--ed5f80783537---------------------bookmark_footer------------------)

Listen

Share

Author: [Eric Latham](https://www.linkedin.com/in/eolatham/) (Software Engineer)

![captionless image](https://miro.medium.com/v2/resize:fit:1400/format:webp/1*vDmHE7Zl8uiYmQvZTm4KDQ.png)

Atmos is our Kubernetes platform that provides the infrastructure needed to manage applications effectively. In this blog, we’ll explain the three-step strategy we used to update our Container Network Interface (CNI) stack. We transitioned from using AWS VPC CNI combined with Calico to Cilium, which offers better performance and reliability for our network needs.

Introduction
------------

As engineers of Atmos, our platform-as-a-service (PaaS) for [Guidewire Cloud](https://medium.com/guidewire-engineering-blog/guidewire-cloud-why-hybrid-tenancy-is-the-right-choice-56a0ff176032), we constantly seek ways to streamline and enhance our architecture. At its core, Atmos offers an abstraction over AWS Elastic Kubernetes Service (EKS), tailored to support Guidewire’s software-as-a-service (SaaS) offerings, including InsuranceSuite. By default, EKS utilizes the [AWS VPC CNI](https://github.com/aws/amazon-vpc-cni-k8s), which until recently lacked support for [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) — a crucial feature for tenant isolation. To address this, we tried using [Calico](https://github.com/projectcalico/calico), but it added significant maintenance overhead due to its complex components and installation process. The challenges posed by this complexity, coupled with a separate initiative to remove [Istio](https://github.com/istio/istio) (previously used for network encryption), led us to explore alternative solutions for our internal networking features. Ultimately, we landed on [Cilium](https://github.com/cilium/cilium), which now provides Atmos:

* An efficient [CNI based on eBPF](https://ebpf.io/what-is-ebpf/)
* Reliable support for NetworkPolicy
* Easy-to-use, high-performance network encryption through [WireGuard](https://www.wireguard.com/)
* The potential to enable additional features and optimizations in the future
* A streamlined networking architecture with significantly less maintenance overhead

Objective
---------

Atmos is deployed on numerous long-lived clusters, and our goal was to replace the CNI across all of them without incurring downtime beyond the regular release maintenance windows. Additionally, we aimed to provide our cluster operators with the ability to roll back the CNI changes if needed.

We achieved this primarily using two fundamental Kubernetes concepts: node selectors and worker node rollouts.

Phase 1: Adding the ability to enable Cilium
--------------------------------------------

During this phase, we completed the majority of the development work needed for the migration. We implemented the following:

* A feature flag to enable or disable Cilium (disabled by default)
* A variable to adjust the size of pre-allocated IP pools on each node when Cilium is enabled

After extensive testing in development environments, we delivered these new capabilities to cluster operators as variables in the Atmos Terraform module, encouraging them to conduct their own tests in staging environments.

### Cilium feature flag

The Cilium feature flag was a critical element of our migration strategy, enabling seamless transitions between the old and new CNIs on any managed cluster as many times as needed.

**Behavior
**Changing the flag would trigger a worker node rollout, resulting in new nodes running the specified CNI while existing nodes would be cycled out without being altered.

_Note:_ We considered [a more complex strategy](https://cilium.io/blog/2020/10/06/skybet-cilium-migration/) that would allow for in-place CNI swaps with no downtime. However, we opted for worker node rollouts instead, believing they offered a safer solution, especially since our regular release maintenance windows provided ample time to manage the required downtime.

**Implementation
**The implementation of the flag involved:

* Adding a node selector to AWS VPC CNI and Calico resources, ensuring they only run on nodes labeled with cilium-enabled=false
* Adding a [Cilium Helm release](https://docs.cilium.io/en/stable/helm-reference/) with a node selector, ensuring it only runs on nodes labeled with cilium-enabled=true
* Adding logic to label nodes with cilium-enabled=true|false based on the Cilium feature flag

We already had existing logic that would trigger a worker node rollout if the node labels changed.

**Rollout
**Our Initial rollout of the Cilium feature flag included several important caveats:

* The release included a mandatory pre-upgrade script to label existing nodes with cilium-enabled=false before applying the Terraform changes. This step was essential to prevent the removal of old CNI pods from existing nodes during the upgrade.
* To ensure that no nodes ran both CNIs simultaneously, cluster operators needed to first upgrade with Cilium disabled before enabling it for their testing.

### Pre-allocated IPs variable

In addition to the Cilium feature flag, we introduced a variable to allow cluster operators to adjust the size of each node’s pre-allocated IP pool (i.e., the number of IPs per node available to be assigned to new pods) when Cilium is enabled.

This feature is essential for our platform, as some clusters require larger IP pools to support larger and more dynamic workloads.

**Implementation
**While the VPC CNI [supports](https://docs.aws.amazon.com/eks/latest/userguide/cni-increase-ip-addresses.html) adjusting pre-allocated IPs using a simple environment variable (WARM_IP_TARGET), Cilium poses some challenges in this regard.

Cilium’s Helm chart doesn’t expose a pre-allocated IPs value directly; it has to be configured with a [custom CNI configuration ConfigMap](https://docs.cilium.io/en/stable/network/concepts/ipam/eni/#custom-eni-configuration).

We implemented this with a local Helm chart for the CNI ConfigMap, which exposes a preallocatedIps value, along with a Terraform variable that controls the preallocatedIps value.

The relevant section of our CNI ConfigMap template looks like this:

```
apiVersion: v1
kind: ConfigMap
metadata:
 name: cni-configuration
 ...
data:
 cni-config: |
   {
     "cniVersion": "x.y.z",
     "name": "cilium",
     "plugins": [       {
         "cniVersion": "x.y.z",
         "type": "cilium-cni",
         "ipam": {
           "pre-allocate": {{ .Values.preallocatedIps }}
         }
       }
       ...
     ]
   }
```

We configured Cilium to use the CNI ConfigMap by setting cni.customConf=true and cni.configMap=cni-configuration in our Cilium Helm release.

Phase 2: Enabling Cilium by default
-----------------------------------

After testing Cilium in lower environments for a complete release cycle to ensure stability, we were ready to initiate the production migration.

This step was straightforward. We only needed to toggle the default value for the Cilium feature flag to “enabled” in the following Atmos release.

Phase 3: Discontinuing the old CNI stack
----------------------------------------

The final step of the migration was certainly the most enjoyable: deleting all the old stuff!

We waited a few release cycles before taking this action to ensure we have enough time to roll back in case any unexpected issues came up after enabling Cilium in production.

Fortunately, we didn’t need to roll back, and we definitely haven’t missed AWS VPC CNI or Calico.

_If you are interested in joining our Engineering teams to develop innovative cloud-distributed systems and large-scale data platforms that enable a wide range of AI/ML SaaS applications, apply at_ [_Guidewire Careers_](https://www.guidewire.com/about/careers?utm_source=medium&utm_medium=referral&utm_campaign=engineering_blog)_._
