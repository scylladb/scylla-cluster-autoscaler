=============================
Scylla Cluster Autoscaler Documentation
=============================

.. toctree::
   :hidden:
   :maxdepth: 2

   generic
   scylla_cluster_autoscaler_crd
   recommender
   updater
   admission_controller

Scylla Cluster Autoscaler is an open source project which helps users of Scylla Open Source and Scylla Enterprise work with Scylla on Kubernetes (K8s).

The Scylla Cluster Autoscaler is an application capable of scaling Scylla database
clusters in an automated manner, freeing the human operator from the task of updating the
required specification manually. Using the elastic scale pattern, the autoscaler manages to
scale the resources both vertically and horizontally.
Basic principle of SCA is that user defines a set of boolean queries on external,
performance-related metrics and the actions to be invoked, were their evaluated values true. It can be also be run in mode in which it produces recommendations, but does not invoke provided scaling actions.
SCA is also equipped with a safety mechanism which prevents sources other than SCA itself scaling controlled resources of a managed cluster.

For the latest status of the project, and reports issue, see the Github Project.


**Choose a topic to begin**:

* :doc:`Deploying Autoscaler on a Kubernetes Cluster <generic>`
* :doc:`Scylla Cluster Autoscaler Custom Resource Definition (CRD) <scylla_cluster_autoscaler_crd>`
* :doc:`Scylla Cluster Autoscaler Recommender component <recommender>`
* :doc:`Scylla Cluster Autoscaler Updater component <updater>`
* :doc:`Scylla Cluster Autoscaler Admission Controller component <admission_controller>`
