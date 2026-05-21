package utilities

import (
	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config Type Maskinporten", func() {

	Describe("DetermineConfigType", func() {
		Context("when client is specified", func() {
			It("should return InlineClient type", func() {
				spec := &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					Client: &accesseratorv1alpha.MaskinportenClientSpec{
						ClientName: "test-client",
					},
				}

				result, err := DetermineConfigType(spec)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(state.InlineClient))
			})
		})

		Context("when clientRef is specified", func() {
			It("should return ClientRef type", func() {
				spec := &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					ClientRef: &accesseratorv1alpha.ResourceRef{
						Name: "existing-client",
					},
				}

				result, err := DetermineConfigType(spec)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(state.ClientRef))
			})
		})

		Context("when secretRef is specified", func() {
			It("should return SecretRef type", func() {
				spec := &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					SecretRef: &accesseratorv1alpha.SecretRef{
						ClientID: accesseratorv1alpha.SecretKeySelector{
							Name: "my-secret",
							Key:  "client-id",
						},
						ClientJWK: accesseratorv1alpha.SecretKeySelector{
							Name: "my-secret",
							Key:  "client-jwk",
						},
					},
				}

				result, err := DetermineConfigType(spec)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(state.SecretRef))
			})
		})

		Context("when multiple config sources are specified", func() {
			It("should return error when client and clientRef are both specified", func() {
				spec := &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					Client: &accesseratorv1alpha.MaskinportenClientSpec{
						ClientName: "test-client",
					},
					ClientRef: &accesseratorv1alpha.ResourceRef{
						Name: "existing-client",
					},
				}

				result, err := DetermineConfigType(spec)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("multiple config sources cannot be used at the same time"))
				Expect(result).To(BeNil())
			})

			It("should return error when client and secretRef are both specified", func() {
				spec := &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					Client: &accesseratorv1alpha.MaskinportenClientSpec{
						ClientName: "test-client",
					},
					SecretRef: &accesseratorv1alpha.SecretRef{
						ClientID: accesseratorv1alpha.SecretKeySelector{
							Name: "my-secret",
							Key:  "client-id",
						},
						ClientJWK: accesseratorv1alpha.SecretKeySelector{
							Name: "my-secret",
							Key:  "client-jwk",
						},
					},
				}

				result, err := DetermineConfigType(spec)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("multiple config sources cannot be used at the same time"))
				Expect(result).To(BeNil())
			})

			It("should return error when clientRef and secretRef are both specified", func() {
				spec := &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
					ClientRef: &accesseratorv1alpha.ResourceRef{
						Name: "existing-client",
					},
					SecretRef: &accesseratorv1alpha.SecretRef{
						ClientID: accesseratorv1alpha.SecretKeySelector{
							Name: "my-secret",
							Key:  "client-id",
						},
						ClientJWK: accesseratorv1alpha.SecretKeySelector{
							Name: "my-secret",
							Key:  "client-jwk",
						},
					},
				}

				result, err := DetermineConfigType(spec)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("multiple config sources cannot be used at the same time"))
				Expect(result).To(BeNil())
			})
		})

		Context("when no config source is specified", func() {
			It("should return None as config type", func() {
				spec := &accesseratorv1alpha.MaskinportenSpec{
					Enabled: true,
				}

				result, err := DetermineConfigType(spec)

				Expect(err).ToNot(HaveOccurred())
				Expect(*result).To(Equal(state.None))
			})
		})
	})

})
